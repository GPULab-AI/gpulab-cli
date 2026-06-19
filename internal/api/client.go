package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
	Debug      bool
}

func NewClient(baseURL, apiKey string, debug bool) *Client {
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		Debug: debug,
	}
}

type APIError struct {
	StatusCode int
	Message    string
	Body       string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("API error (HTTP %d): %s", e.StatusCode, e.Body)
}

func (c *Client) do(method, path string, body interface{}) ([]byte, error) {
	url := c.BaseURL + path

	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("api-key", c.APIKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if c.Debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] %s %s\n", method, url)
		if body != nil {
			jsonData, _ := json.MarshalIndent(body, "", "  ")
			fmt.Fprintf(os.Stderr, "[DEBUG] Request body: %s\n", string(jsonData))
		}
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if c.Debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] Response %d: %s\n", resp.StatusCode, string(respBody))
	}

	if resp.StatusCode >= 400 {
		var errResp struct {
			Message string `json:"message"`
		}
		json.Unmarshal(respBody, &errResp)
		return respBody, &APIError{
			StatusCode: resp.StatusCode,
			Message:    errResp.Message,
			Body:       string(respBody),
		}
	}

	return respBody, nil
}

// doRaw issues a request with an arbitrary body and content type (e.g. multipart
// uploads). Unlike do, it does not JSON-encode the body.
func (c *Client) doRaw(method, path, contentType string, body io.Reader) ([]byte, error) {
	url := c.BaseURL + path

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("api-key", c.APIKey)
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	if c.Debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] %s %s (%s)\n", method, url, contentType)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if c.Debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] Response %d (%d bytes)\n", resp.StatusCode, len(respBody))
	}

	if resp.StatusCode >= 400 {
		var errResp struct {
			Message string `json:"message"`
			Error   string `json:"error"`
		}
		json.Unmarshal(respBody, &errResp)
		message := errResp.Message
		if message == "" {
			message = errResp.Error
		}
		return respBody, &APIError{
			StatusCode: resp.StatusCode,
			Message:    message,
			Body:       string(respBody),
		}
	}

	return respBody, nil
}

func (c *Client) Get(path string) ([]byte, error) {
	return c.do("GET", path, nil)
}

func (c *Client) Post(path string, body interface{}) ([]byte, error) {
	return c.do("POST", path, body)
}

func (c *Client) Put(path string, body interface{}) ([]byte, error) {
	return c.do("PUT", path, body)
}

func (c *Client) Delete(path string) ([]byte, error) {
	return c.do("DELETE", path, nil)
}

// WithTimeout returns a new client with custom timeout
func (c *Client) WithTimeout(d time.Duration) *Client {
	return &Client{
		BaseURL: c.BaseURL,
		APIKey:  c.APIKey,
		HTTPClient: &http.Client{
			Timeout: d,
		},
		Debug: c.Debug,
	}
}
