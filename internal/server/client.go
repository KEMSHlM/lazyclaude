package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const clientTimeout = 5 * time.Second

// Client is a thin HTTP client for the lazyclaude MCP server API.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// NewClient creates a client targeting the server at the given port with the given auth token.
func NewClient(port int, token string) *Client {
	return &Client{
		baseURL: fmt.Sprintf("http://127.0.0.1:%d", port),
		token:   token,
		http:    &http.Client{Timeout: clientTimeout},
	}
}

// Sessions fetches the session list from GET /msg/sessions.
func (c *Client) Sessions() ([]SessionInfo, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/msg/sessions", nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("X-Auth-Token", c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request /msg/sessions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, bytes.TrimSpace(body))
	}

	var sessions []SessionInfo
	if err := json.NewDecoder(resp.Body).Decode(&sessions); err != nil {
		return nil, fmt.Errorf("decode sessions: %w", err)
	}
	return sessions, nil
}

// SendMessage sends a message via POST /msg/send.
func (c *Client) SendMessage(from, to, msgType, body string) error {
	payload := msgSendRequest{
		From: from,
		To:   to,
		Type: msgType,
		Body: body,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal send request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/msg/send", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("X-Auth-Token", c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request /msg/send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, bytes.TrimSpace(body))
	}
	return nil
}
