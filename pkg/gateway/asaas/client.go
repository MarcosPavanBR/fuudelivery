package asaas

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/carloshomar/fuudelivery/pkg/gateway"
)

// Client é o cliente HTTP para a API do Asaas.
type Client struct {
	baseURL     string
	apiKey      string
	httpClient  *http.Client
	maxRetries  int
	retryDelay  time.Duration
}

// NewClient cria um novo cliente Asaas.
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
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
	return c.doRequest("POST", path, body, nil)
}

// postWithHeaders envia uma requisição POST com headers customizados.
func (c *Client) postWithHeaders(path string, body interface{}, headers map[string]string) ([]byte, error) {
	return c.doRequest("POST", path, body, headers)
}

// get envia uma requisição GET com retry.
func (c *Client) get(path string) ([]byte, error) {
	return c.doRequest("GET", path, nil, nil)
}

// put envia uma requisição PUT com retry.
func (c *Client) put(path string, body interface{}) ([]byte, error) {
	return c.doRequest("PUT", path, body, nil)
}

// doRequest executa uma requisição HTTP com retry e backoff exponencial.
func (c *Client) doRequest(method, path string, body interface{}, extraHeaders map[string]string) ([]byte, error) {
	var bodyReader io.Reader

	if body != nil {
		jsonBytes, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("asaas: failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBytes)
	}

	var lastErr error
	for attempt := 1; attempt <= c.maxRetries; attempt++ {
		url := c.baseURL + path

		req, err := http.NewRequest(method, url, bodyReader)
		if err != nil {
			return nil, fmt.Errorf("asaas: failed to create request: %w", err)
		}

		// Headers de autenticação
		req.Header.Set("access_token", c.apiKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		// Headers customizados (ex: Idempotency-Key)
		for k, v := range extraHeaders {
			req.Header.Set(k, v)
		}

		// Retry: resetar body reader
		if body != nil && attempt > 1 {
			jsonBytes, _ := json.Marshal(body)
			bodyReader = bytes.NewReader(jsonBytes)
			req.Body = io.NopCloser(bodyReader)
		}

		log.Printf("[ASAAS] %s %s (attempt %d/%d)", method, path, attempt, c.maxRetries)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("asaas: request failed: %w", err)
			log.Printf("[ASAAS] Request failed: %v", err)
			time.Sleep(c.retryDelay * time.Duration(attempt))
			continue
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = fmt.Errorf("asaas: failed to read response body: %w", err)
			time.Sleep(c.retryDelay * time.Duration(attempt))
			continue
		}

		// Sucesso (2xx)
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			log.Printf("[ASAAS] %s %s → %d (OK)", method, path, resp.StatusCode)
			return respBody, nil
		}

		// Erro 4xx (não retry)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return nil, fmt.Errorf("asaas: API error %d: %s", resp.StatusCode, string(respBody))
		}

		// Erro 5xx (retry)
		lastErr = fmt.Errorf("asaas: server error %d: %s", resp.StatusCode, string(respBody))
		log.Printf("[ASAAS] Server error %d, retrying...", resp.StatusCode)
		time.Sleep(c.retryDelay * time.Duration(attempt))
	}

	return nil, fmt.Errorf("asaas: max retries exceeded: %w", lastErr)
}
