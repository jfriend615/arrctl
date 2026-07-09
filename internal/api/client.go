package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

func New(baseURL, key string) *Client {
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), APIKey: key, HTTP: &http.Client{Timeout: 20 * time.Second}}
}

func (c *Client) Do(ctx context.Context, method, endpoint string, in any, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+endpoint, body)
	if err != nil {
		return err
	}
	req.Header.Set("X-Api-Key", c.APIKey)
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		if resp.StatusCode == http.StatusUnauthorized {
			return fmt.Errorf("Authentication failed")
		}
		if resp.StatusCode == http.StatusNotFound {
			return fmt.Errorf("Not found: %s", endpoint)
		}
		return fmt.Errorf("API request failed (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	if out != nil && len(b) > 0 {
		return json.Unmarshal(b, out)
	}
	return nil
}

func (c *Client) Tautulli(ctx context.Context, cmd string, params map[string]string, out any) error {
	q := url.Values{}
	q.Set("apikey", c.APIKey)
	q.Set("cmd", cmd)
	for k, v := range params {
		q.Set(k, v)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/api/v2?"+q.Encode(), nil)
	if err != nil {
		return err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("Tautulli request failed (HTTP %d)", resp.StatusCode)
	}
	if out != nil {
		return json.Unmarshal(b, out)
	}
	return nil
}
