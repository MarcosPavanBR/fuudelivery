package pagarme

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

// ═══════════════════════════════════════════════════════════════
// CLIENT HTTP
// ═══════════════════════════════════════════════════════════════

// Client é o cliente HTTP para a API do Pagar.me v4.
//
// Suporta:
//   - Retry com backoff exponencial (3 tentativas)
//   - Timeout configurável por requisição
//   - Validação de response status
//   - Logging de requests/responses para auditoria
//
// Segurança:
//   - API Key enviada via header Authorization: Bearer {api_key}
//   - Nunca loga a API Key completa
type Client struct {
	apiKey       string
	encryptionKey string
	baseURL      string
	httpClient   *http.Client
	maxRetries   int
	retryDelay   time.Duration
}

// NewClient cria um novo cliente Pagar.me.
//
// Lê as env vars:
//   - PAGARME_API_KEY (obrigatório)
//   - PAGARME_ENCRYPTION_KEY (obrigatório para cartão)
//
// Retorna erro se PAGARME_API_KEY não estiver configurado.
func NewClient() (*Client, error) {
	apiKey := os.Getenv("PAGARME_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("pagarme: PAGARME_API_KEY not configured")
	}

	return &Client{
		apiKey:        apiKey,
		encryptionKey: os.Getenv("PAGARME_ENCRYPTION_KEY"),
		baseURL:       "https://api.pagar.me/core/v5",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		maxRetries: 3,
		retryDelay: 1 * time.Second,
	}, nil
}

// NewClientWithConfig cria um cliente com configuração customizada.
func NewClientWithConfig(apiKey, encryptionKey, baseURL string, timeout time.Duration) *Client {
	return &Client{
		apiKey:        apiKey,
		encryptionKey: encryptionKey,
		baseURL:       baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		maxRetries: 3,
		retryDelay: 1 * time.Second,
	}
}

// ═══════════════════════════════════════════════════════════════
// MÉTODOS HTTP
// ═══════════════════════════════════════════════════════════════

// post envia uma requisição POST com retry.
func (c *Client) post(path string, body interface{}) ([]byte, error) {
	return c.doRequest("POST", path, body)
}

// get envia uma requisição GET com retry.
func (c *Client) get(path string) ([]byte, error) {
	return c.doRequest("GET", path, nil)
}

// put envia uma requisição PUT com retry.
func (c *Client) put(path string, body interface{}) ([]byte, error) {
	return c.doRequest("PUT", path, body)
}

// doRequest executa uma requisição HTTP com retry e backoff exponencial.
func (c *Client) doRequest(method, path string, body interface{}) ([]byte, error) {
	var bodyReader io.Reader

	if body != nil {
		jsonBytes, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("pagarme: failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBytes)
	}

	var lastErr error
	for attempt := 1; attempt <= c.maxRetries; attempt++ {
		url := c.baseURL + path

		req, err := http.NewRequest(method, url, bodyReader)
		if err != nil {
			return nil, fmt.Errorf("pagarme: failed to create request: %w", err)
		}

		// Headers de autenticação
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		// Retry: resetar body reader se necessário
		if body != nil && attempt > 1 {
			jsonBytes, _ := json.Marshal(body)
			bodyReader = bytes.NewReader(jsonBytes)
			req.Body = io.NopCloser(bodyReader)
		}

		log.Printf("[PAGARME] %s %s (attempt %d/%d)", method, path, attempt, c.maxRetries)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("pagarme: request failed: %w", err)
			log.Printf("[PAGARME] Request failed: %v", err)
			time.Sleep(c.retryDelay * time.Duration(attempt))
			continue
		}

		defer resp.Body.Close()
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = fmt.Errorf("pagarme: failed to read response body: %w", err)
			time.Sleep(c.retryDelay * time.Duration(attempt))
			continue
		}

		// Sucesso (2xx)
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			log.Printf("[PAGARME] %s %s → %d (OK)", method, path, resp.StatusCode)
			return respBody, nil
		}

		// Erro 4xx (não retry)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return nil, fmt.Errorf("pagarme: API error %d: %s", resp.StatusCode, string(respBody))
		}

		// Erro 5xx (retry)
		lastErr = fmt.Errorf("pagarme: server error %d: %s", resp.StatusCode, string(respBody))
		log.Printf("[PAGARME] Server error %d, retrying...", resp.StatusCode)
		time.Sleep(c.retryDelay * time.Duration(attempt))
	}

	return nil, fmt.Errorf("pagarme: max retries exceeded: %w", lastErr)
}
