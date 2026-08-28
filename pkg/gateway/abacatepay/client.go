package abacatepay

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

// Client é o cliente HTTP para a API do AbacatePay.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	maxRetries int
	retryDelay time.Duration
}

// NewClient cria um novo cliente AbacatePay.
func NewClient() (*Client, error) {
	apiKey := os.Getenv("ABACATE_PAY_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("abacatepay: ABACATE_PAY_API_KEY not configured")
	}

	return &Client{
		apiKey:  apiKey,
		baseURL: "https://api.abacatepay.com/v1",
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

// doRequest executa uma requisição HTTP com retry.
func (c *Client) doRequest(method, path string, body interface{}) ([]byte, error) {
	var bodyReader io.Reader

	if body != nil {
		jsonBytes, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("abacatepay: failed to marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBytes)
	}

	var lastErr error
	for attempt := 1; attempt <= c.maxRetries; attempt++ {
		url := c.baseURL + path

		req, err := http.NewRequest(method, url, bodyReader)
		if err != nil {
			return nil, fmt.Errorf("abacatepay: failed to create request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		if body != nil && attempt > 1 {
			jsonBytes, _ := json.Marshal(body)
			bodyReader = bytes.NewReader(jsonBytes)
			req.Body = io.NopCloser(bodyReader)
		}

		log.Printf("[ABACATEPAY] %s %s (attempt %d/%d)", method, path, attempt, c.maxRetries)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("abacatepay: request failed: %w", err)
			time.Sleep(c.retryDelay * time.Duration(attempt))
			continue
		}

		defer resp.Body.Close()
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = fmt.Errorf("abacatepay: failed to read response: %w", err)
			time.Sleep(c.retryDelay * time.Duration(attempt))
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			log.Printf("[ABACATEPAY] %s %s → %d (OK)", method, path, resp.StatusCode)
			return respBody, nil
		}

		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return nil, fmt.Errorf("abacatepay: API error %d: %s", resp.StatusCode, string(respBody))
		}

		lastErr = fmt.Errorf("abacatepay: server error %d: %s", resp.StatusCode, string(respBody))
		time.Sleep(c.retryDelay * time.Duration(attempt))
	}

	return nil, fmt.Errorf("abacatepay: max retries exceeded: %w", lastErr)
}

// postWithHeaders envia uma requisição POST com headers customizados.
func (c *Client) postWithHeaders(path string, body interface{}, headers map[string]string) ([]byte, error) {
	return c.doRequestWithHeaders("POST", path, body, headers)
}

// doRequestWithHeaders envia requisição HTTP com headers customizados e retry.
func (c *Client) doRequestWithHeaders(method, path string, body interface{}, headers map[string]string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt < c.maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(c.retryDelay * time.Duration(attempt))
		}

		reqBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}

		req, err := http.NewRequest(method, c.baseURL+path, bytes.NewReader(reqBody))
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("read response: %w", err)
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return respBody, nil
		}

		lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}
