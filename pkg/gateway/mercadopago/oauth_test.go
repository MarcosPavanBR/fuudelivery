package mercadopago

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func testConfig(apiBase string) OAuthConfig {
	return OAuthConfig{
		ClientID:     "app-123",
		ClientSecret: "secret-xyz",
		RedirectURI:  "https://fuu.example/gateways/mercadopago/callback",
		APIBaseURL:   apiBase,
		AuthBaseURL:  "https://auth.example/authorization",
	}
}

func TestAuthorizeURL(t *testing.T) {
	cfg := testConfig("")
	got := cfg.AuthorizeURL("estado-assinado-abc")

	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("URL inválida: %v", err)
	}
	q := u.Query()
	casos := map[string]string{
		"client_id":     "app-123",
		"response_type": "code",
		"platform_id":   "mp",
		"redirect_uri":  "https://fuu.example/gateways/mercadopago/callback",
		"state":         "estado-assinado-abc",
	}
	for k, want := range casos {
		if got := q.Get(k); got != want {
			t.Errorf("query %q: obtive %q, queria %q", k, got, want)
		}
	}
	if !strings.HasPrefix(got, "https://auth.example/authorization?") {
		t.Errorf("base errada: %s", got)
	}
}

// TestExchange verifica que o code vira tokens e que o request bate o contrato:
// POST /oauth/token com grant_type=authorization_code e as credenciais do app.
func TestExchange(t *testing.T) {
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/token" {
			t.Errorf("path: obtive %s, queria /oauth/token", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("método: obtive %s, queria POST", r.Method)
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"access_token":"APP_USR-access-1",
			"refresh_token":"TG-refresh-1",
			"user_id":998877,
			"expires_in":15552000,
			"scope":"read write",
			"token_type":"bearer"
		}`))
	}))
	defer srv.Close()

	cfg := testConfig(srv.URL)
	toks, err := cfg.Exchange(context.Background(), "code-do-callback")
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}

	if gotBody["grant_type"] != "authorization_code" {
		t.Errorf("grant_type: obtive %q", gotBody["grant_type"])
	}
	if gotBody["code"] != "code-do-callback" {
		t.Errorf("code: obtive %q", gotBody["code"])
	}
	if gotBody["client_id"] != "app-123" || gotBody["client_secret"] != "secret-xyz" {
		t.Errorf("credenciais do app não foram enviadas: %+v", gotBody)
	}
	if gotBody["redirect_uri"] != cfg.RedirectURI {
		t.Errorf("redirect_uri: obtive %q", gotBody["redirect_uri"])
	}

	if toks.AccessToken != "APP_USR-access-1" || toks.RefreshToken != "TG-refresh-1" {
		t.Errorf("tokens: %+v", toks)
	}
	if toks.UserID != 998877 || toks.MPUserIDString() != "998877" {
		t.Errorf("user_id: %d / %q", toks.UserID, toks.MPUserIDString())
	}
	if toks.ExpiresIn != 15552000 {
		t.Errorf("expires_in: %d", toks.ExpiresIn)
	}
	// ExpiresAt tem que cair no futuro (≈180 dias), nunca no passado.
	if !toks.ExpiresAt().After(toks.ExpiresAt().Add(-time.Second)) {
		t.Error("ExpiresAt deveria estar no futuro")
	}
}

// TestRefresh verifica o grant_type=refresh_token e que o refresh_token novo
// volta (o MP rotaciona o refresh a cada uso).
func TestRefresh(t *testing.T) {
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		_, _ = w.Write([]byte(`{
			"access_token":"APP_USR-access-2",
			"refresh_token":"TG-refresh-2",
			"user_id":998877,
			"expires_in":15552000,
			"token_type":"bearer"
		}`))
	}))
	defer srv.Close()

	cfg := testConfig(srv.URL)
	toks, err := cfg.Refresh(context.Background(), "TG-refresh-1")
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if gotBody["grant_type"] != "refresh_token" {
		t.Errorf("grant_type: obtive %q", gotBody["grant_type"])
	}
	if gotBody["refresh_token"] != "TG-refresh-1" {
		t.Errorf("refresh_token enviado: obtive %q", gotBody["refresh_token"])
	}
	if toks.RefreshToken != "TG-refresh-2" {
		t.Errorf("o refresh_token novo tem que voltar para ser regravado: obtive %q", toks.RefreshToken)
	}
}

// TestExchange_ErroDoMP: status não-2xx vira erro, sem vazar o corpo cru.
func TestExchange_ErroDoMP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"code expirado"}`))
	}))
	defer srv.Close()

	cfg := testConfig(srv.URL)
	_, err := cfg.Exchange(context.Background(), "code-velho")
	if err == nil {
		t.Fatal("Exchange com erro do MP devia falhar")
	}
	if !strings.Contains(err.Error(), "code expirado") {
		t.Errorf("erro devia conter a descrição do MP: %v", err)
	}
}

// TestExchange_RespostaIncompleta: 2xx mas sem tokens é erro — nunca gravar um
// recebedor "ativo" sem token.
func TestExchange_RespostaIncompleta(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"user_id":123,"expires_in":100}`))
	}))
	defer srv.Close()

	cfg := testConfig(srv.URL)
	if _, err := cfg.Exchange(context.Background(), "code"); err == nil {
		t.Error("resposta sem access_token/refresh_token devia falhar")
	}
}
