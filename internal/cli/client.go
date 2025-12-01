package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) Health() (map[string]interface{}, error) {
	return c.get("/api/v1/health")
}

func (c *Client) ListHosts() (map[string]interface{}, error) {
	return c.get("/api/v1/hosts")
}

func (c *Client) GetHost(hostname string, limit int) (map[string]interface{}, error) {
	url := fmt.Sprintf("/api/v1/hosts/%s", hostname)
	if limit > 0 {
		url = fmt.Sprintf("%s?limit=%d", url, limit)
	}
	return c.get(url)
}

func (c *Client) GetStats() (map[string]interface{}, error) {
	return c.get("/api/v1/stats")
}

func (c *Client) get(path string) (map[string]interface{}, error) {
	url := c.baseURL + path

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return result, nil
}
