package mercadopago

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

// Client é o cliente HTTP para a API do Mercado Pago.
type Client struct {
	accessToken string
	baseURL     string
	httpClient  *http.Client
	maxRetries  int
	retryDelay  time.Duration
}

// NewClient cria um novo cliente Mercado Pago.
func NewClient() (*Client, error) {
	accessToken := os.Getenv("MERCADOPAGO_ACCESS_TOKEN")
	if accessToken == "" {
		return nil, fmt.Errorf("mercadopago: MERCADOPAGO_ACCESS_TOKEN not configured")
	}

	return &Client{
		accessToken: accessToken,
		baseURL:     "https://api.mercadopago.com/v1",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		maxRetries: 3,
		retryDelay: 1 * time.Second,
	}, nil
}

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

// delete envia uma requisição DELETE com retry.
func (c *Client) delete(path string) ([]byte, error) {
	return c.doRequest("DELETE", path, nil)
}

// doRequest executa uma requisição HTTP com retry.
func (c *Client) doRequest(method, path string, body interface{}) ([]byte, error) {
	var bodyReader io.Reader

	if body != nil {
		jsonBytes, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("mercadopago: failed to marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBytes)
	}

	var lastErr error
	for attempt := 1; attempt <= c.maxRetries; attempt++ {
		url := c.baseURL + path

		req, err := http.NewRequest(method, url, bodyReader)
		if err != nil {
			return nil, fmt.Errorf("mercadopago: failed to create request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+c.accessToken)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		if body != nil && attempt > 1 {
			jsonBytes, _ := json.Marshal(body)
			bodyReader = bytes.NewReader(jsonBytes)
			req.Body = io.NopCloser(bodyReader)
		}

		log.Printf("[MERCADOPAGO] %s %s (attempt %d/%d)", method, path, attempt, c.maxRetries)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("mercadopago: request failed: %w", err)
			time.Sleep(c.retryDelay * time.Duration(attempt))
			continue
		}

		defer resp.Body.Close()
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = fmt.Errorf("mercadopago: failed to read response: %w", err)
			time.Sleep(c.retryDelay * time.Duration(attempt))
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			log.Printf("[MERCADOPAGO] %s %s → %d (OK)", method, path, resp.StatusCode)
			return respBody, nil
		}

		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return nil, fmt.Errorf("mercadopago: API error %d: %s", resp.StatusCode, string(respBody))
		}

		lastErr = fmt.Errorf("mercadopago: server error %d: %s", resp.StatusCode, string(respBody))
		time.Sleep(c.retryDelay * time.Duration(attempt))
	}

	return nil, fmt.Errorf("mercadopago: max retries exceeded: %w", lastErr)
}
