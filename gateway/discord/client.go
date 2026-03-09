package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	discordAPIBase     = "https://discord.com/api/v10"
	defaultHTTPTimeout = 10 * time.Second
)

// Client is a minimal Discord REST API client.
// All requests include the required Bot token and User-Agent headers.
type Client struct {
	token   string
	baseURL string // overridable for tests via newTestClient
	http    *http.Client
}

// NewClient constructs a REST client authenticated with the given bot token.
func NewClient(token string) *Client {
	return &Client{
		token:   token,
		baseURL: discordAPIBase,
		http:    &http.Client{Timeout: defaultHTTPTimeout},
	}
}

// newTestClient constructs a Client pointing at a custom base URL (e.g. httptest.Server).
func newTestClient(token, baseURL string) *Client {
	return &Client{
		token:   token,
		baseURL: baseURL,
		http:    &http.Client{Timeout: defaultHTTPTimeout},
	}
}

// GetGatewayURL retrieves the recommended WebSocket gateway URL for this bot.
// Discord returns {"url": "wss://gateway.discord.gg", "shards": N, ...}.
func (c *Client) GetGatewayURL(ctx context.Context) (string, error) {
	var result struct {
		URL string `json:"url"`
	}
	if err := c.get(ctx, "/gateway/bot", &result); err != nil {
		return "", err
	}
	return result.URL, nil
}

// SendMessage posts a text message to the given Discord channel.
func (c *Client) SendMessage(ctx context.Context, channelID, text string) (*Message, error) {
	var msg Message
	path := fmt.Sprintf("/channels/%s/messages", channelID)
	if err := c.post(ctx, path, CreateMessageRequest{Content: text}, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// ---- internal helpers -------------------------------------------------------

func (c *Client) get(ctx context.Context, path string, result any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, http.NoBody)
	if err != nil {
		return fmt.Errorf("discord: build GET %s: %w", path, err)
	}
	return c.do(req, path, result)
}

func (c *Client) post(ctx context.Context, path string, body, result any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("discord: marshal %s body: %w", path, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("discord: build POST %s: %w", path, err)
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req, path, result)
}

func (c *Client) do(req *http.Request, path string, result any) error {
	req.Header.Set("Authorization", "Bot "+c.token)
	req.Header.Set("User-Agent", "AetherCore (https://github.com/fzihak/aethercore, v0.1.0)")

	resp, err := c.http.Do(req) // #nosec G704 — URL is a fixed API base, not user-tainted
	if err != nil {
		return fmt.Errorf("discord: %s: %w", path, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("discord: read %s response: %w", path, err)
	}

	if resp.StatusCode >= 400 {
		var apiErr APIError
		if jsonErr := json.Unmarshal(raw, &apiErr); jsonErr == nil && apiErr.Message != "" {
			return &apiErr
		}
		return fmt.Errorf("discord: %s returned HTTP %d", path, resp.StatusCode)
	}

	if result != nil {
		if err := json.Unmarshal(raw, result); err != nil {
			return fmt.Errorf("discord: decode %s result: %w", path, err)
		}
	}
	return nil
}
