package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	// telegramAPIBase is the base URL for all Telegram Bot API calls.
	telegramAPIBase = "https://api.telegram.org/bot"

	// defaultHTTPTimeout is the client-level timeout for non-polling requests.
	// Long-poll requests use a per-request context deadline instead.
	defaultHTTPTimeout = 10 * time.Second
)

// Client is a thin, stdlib-only Telegram Bot API HTTP client.
// All methods are safe for concurrent use from multiple goroutines.
type Client struct {
	token   string
	baseURL string // overridable for tests
	http    *http.Client
}

// NewClient constructs a Client for the given bot token.
func NewClient(token string) *Client {
	return &Client{
		token:   token,
		baseURL: telegramAPIBase + token,
		http:    &http.Client{Timeout: defaultHTTPTimeout},
	}
}

// newTestClient constructs a Client whose base URL points at a test server.
// Exported only for use within this package's tests.
func newTestClient(token, baseURL string) *Client {
	return &Client{
		token:   token,
		baseURL: baseURL,
		http:    &http.Client{Timeout: defaultHTTPTimeout},
	}
}

// GetMe returns the bot's own User record; used to verify the token on startup.
func (c *Client) GetMe(ctx context.Context) (*User, error) {
	var u User
	if err := c.get(ctx, "getMe", &u); err != nil {
		return nil, err
	}
	return &u, nil
}

// GetUpdates fetches pending updates from Telegram via long-polling.
// offset should be set to the last received UpdateID + 1 on subsequent calls.
// pollTimeout is the server-side long-poll window in seconds (max 50).
func (c *Client) GetUpdates(ctx context.Context, offset int64, pollTimeout int) ([]Update, error) {
	req := getUpdatesRequest{
		Offset:  offset,
		Limit:   100,
		Timeout: pollTimeout,
	}
	var updates []Update
	if err := c.post(ctx, "getUpdates", req, &updates); err != nil {
		return nil, err
	}
	return updates, nil
}

// SendMessage sends a text message to chatID.
func (c *Client) SendMessage(ctx context.Context, chatID int64, text string) (*Message, error) {
	req := SendMessageRequest{
		ChatID:    chatID,
		Text:      text,
		ParseMode: "Markdown",
	}
	var msg Message
	if err := c.post(ctx, "sendMessage", req, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// -----------------------------------------------------------------------------
// internal helpers
// -----------------------------------------------------------------------------

// get executes a parameter-less GET to the given method and decodes the result.
func (c *Client) get(ctx context.Context, method string, result any) error {
	url := c.baseURL + "/" + method
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return fmt.Errorf("telegram: build GET %s: %w", method, err)
	}
	return c.do(req, method, result)
}

// post JSON-encodes body, POSTs to method, and decodes the result.
func (c *Client) post(ctx context.Context, method string, body, result any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("telegram: marshal %s body: %w", method, err)
	}

	url := c.baseURL + "/" + method
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("telegram: build POST %s: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req, method, result)
}

// do executes req, checks the API envelope for ok=true, and decodes result.
func (c *Client) do(req *http.Request, method string, result any) error {
	resp, err := c.http.Do(req) // #nosec G704 -- URL built from configured baseURL; method is always a const string
	if err != nil {
		return fmt.Errorf("telegram: %s: %w", method, err)
	}
	defer resp.Body.Close()

	var envelope apiResponse[json.RawMessage]
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("telegram: decode %s response: %w", method, err)
	}
	if !envelope.OK {
		return &APIError{Code: envelope.ErrorCode, Description: envelope.Description}
	}
	if result != nil {
		if err := json.Unmarshal(envelope.Result, result); err != nil {
			return fmt.Errorf("telegram: decode %s result: %w", method, err)
		}
	}
	return nil
}
