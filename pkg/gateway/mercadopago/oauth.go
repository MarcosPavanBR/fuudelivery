package mercadopago

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// OAuth do Mercado Pago — o vínculo que faz o split na origem funcionar.
//
// O modelo de marketplace exige que cada estabelecimento autorize a plataforma
// a cobrar em nome dele. O fluxo é OAuth authorization-code:
//
//  1. AuthorizeURL(state) → o lojista é redirecionado ao MP, loga na conta
//     DELE e autoriza.
//  2. O MP redireciona de volta com um `code` (válido ~10 min).
//  3. Exchange(code) troca o code por access_token + refresh_token + user_id.
//  4. A partir daí, cobranças criadas com o access_token do vendedor caem
//     DIRETO na conta dele; a plataforma só fica com a application_fee.
//
// O access_token vale ~180 dias; Refresh renova antes disso — e o MP TAMBÉM
// devolve um refresh_token novo a cada refresh, que precisa ser regravado.
//
// Contrato confirmado na doc oficial (set/2026):
//   - POST https://api.mercadopago.com/oauth/token  (fora do /v1)
//   - campos: access_token, refresh_token, user_id, expires_in, scope, token_type
// A URL de AUTORIZAÇÃO (onde o lojista loga) é configurável porque o host varia
// por país/versão; erra-la manda o lojista pra lugar nenhum, mas é config, não
// deploy. Default abaixo é o do Brasil.

const (
	defaultAuthBaseURL = "https://auth.mercadopago.com.br/authorization"
	defaultAPIBaseURL  = "https://api.mercadopago.com"
)

// OAuthConfig carrega as credenciais do APLICATIVO de marketplace da plataforma
// (não de um vendedor). ClientID/Secret identificam o app; RedirectURI tem que
// bater exatamente com o cadastrado no painel do MP.
type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string

	// AuthBaseURL e APIBaseURL têm defaults de produção; sobrescritos em teste
	// (para o httptest) e configuráveis em produção se o host mudar.
	AuthBaseURL string
	APIBaseURL  string

	HTTPClient *http.Client
}

// OAuthTokens é a resposta do /oauth/token, normalizada.
type OAuthTokens struct {
	AccessToken  string
	RefreshToken string
	UserID       int64
	ExpiresIn    int // segundos
	Scope        string
	TokenType    string
}

// ExpiresAt converte ExpiresIn (relativo) num instante absoluto, a partir de
// agora. É o que vai para token_expires_at no banco.
func (t OAuthTokens) ExpiresAt() time.Time {
	return time.Now().Add(time.Duration(t.ExpiresIn) * time.Second)
}

func (cfg OAuthConfig) authBase() string {
	if cfg.AuthBaseURL != "" {
		return cfg.AuthBaseURL
	}
	return defaultAuthBaseURL
}

func (cfg OAuthConfig) apiBase() string {
	if cfg.APIBaseURL != "" {
		return cfg.APIBaseURL
	}
	return defaultAPIBaseURL
}

func (cfg OAuthConfig) httpClient() *http.Client {
	if cfg.HTTPClient != nil {
		return cfg.HTTPClient
	}
	return &http.Client{Timeout: 30 * time.Second}
}

// AuthorizeURL monta a URL para onde o lojista é enviado para autorizar.
//
// `state` é o anti-CSRF: quem chama gera um valor assinado amarrado ao
// estabelecimento e confere na volta. Nunca deixe vazio.
func (cfg OAuthConfig) AuthorizeURL(state string) string {
	q := url.Values{}
	q.Set("client_id", cfg.ClientID)
	q.Set("response_type", "code")
	q.Set("platform_id", "mp")
	q.Set("redirect_uri", cfg.RedirectURI)
	q.Set("state", state)
	return cfg.authBase() + "?" + q.Encode()
}

// tokenResponse é o corpo cru do /oauth/token.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	UserID       int64  `json:"user_id"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	TokenType    string `json:"token_type"`
	// Campos de erro do MP, quando o status não é 2xx.
	Err     string `json:"error"`
	ErrDesc string `json:"error_description"`
	Message string `json:"message"`
}

// Exchange troca o authorization code por tokens (grant_type=authorization_code).
func (cfg OAuthConfig) Exchange(ctx context.Context, code string) (*OAuthTokens, error) {
	return cfg.tokenRequest(ctx, map[string]string{
		"client_id":     cfg.ClientID,
		"client_secret": cfg.ClientSecret,
		"grant_type":    "authorization_code",
		"code":          code,
		"redirect_uri":  cfg.RedirectURI,
	})
}

// Refresh renova o access_token usando o refresh_token
// (grant_type=refresh_token). O MP devolve um refresh_token NOVO — o chamador
// tem que regravar os dois.
func (cfg OAuthConfig) Refresh(ctx context.Context, refreshToken string) (*OAuthTokens, error) {
	return cfg.tokenRequest(ctx, map[string]string{
		"client_id":     cfg.ClientID,
		"client_secret": cfg.ClientSecret,
		"grant_type":    "refresh_token",
		"refresh_token": refreshToken,
	})
}

func (cfg OAuthConfig) tokenRequest(ctx context.Context, payload map[string]string) (*OAuthTokens, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("mercadopago oauth: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.apiBase()+"/oauth/token", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("mercadopago oauth: criar request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := cfg.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("mercadopago oauth: request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("mercadopago oauth: ler resposta: %w", err)
	}

	var tr tokenResponse
	if err := json.Unmarshal(raw, &tr); err != nil {
		return nil, fmt.Errorf("mercadopago oauth: resposta inválida (HTTP %d): %w", resp.StatusCode, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Não vaza o corpo cru (pode conter fragmento de credencial); reporta o
		// que o MP mandou de descrição de erro.
		desc := tr.ErrDesc
		if desc == "" {
			desc = tr.Message
		}
		if desc == "" {
			desc = tr.Err
		}
		return nil, fmt.Errorf("mercadopago oauth: HTTP %d: %s", resp.StatusCode, desc)
	}

	if tr.AccessToken == "" || tr.RefreshToken == "" {
		return nil, fmt.Errorf("mercadopago oauth: resposta sem access_token/refresh_token")
	}

	return &OAuthTokens{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		UserID:       tr.UserID,
		ExpiresIn:    tr.ExpiresIn,
		Scope:        tr.Scope,
		TokenType:    tr.TokenType,
	}, nil
}

// MPUserIDString devolve o user_id como string (o formato que gravamos em
// recipients.mp_user_id e gateway_recipient_id).
func (t OAuthTokens) MPUserIDString() string {
	return strconv.FormatInt(t.UserID, 10)
}
