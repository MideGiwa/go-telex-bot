package telex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client provides methods for interacting with the Telex API.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// Message represents a Telex message.
type Message struct {
	ID        string    `json:"id"`
	ChannelID string    `json:"channel_id"`
	SenderID  string    `json:"sender_id"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	// Add other fields from Telex message structure as needed
}

// WebhookEvent represents the structure of a Telex webhook event.
type WebhookEvent struct {
	EventType string  `json:"event_type"` // e.g., "message.created"
	Payload   Message `json:"payload"`
}

// NewClient initializes and returns a new Telex API client.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SendMessage sends a message to a specified Telex channel.
func (c *Client) SendMessage(ctx context.Context, channelID, text string) error {
	url := fmt.Sprintf("%s/channels/%s/messages", c.baseURL, channelID)

	payload := map[string]string{
		"text": text,
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal message payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request to Telex: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	// Assuming Telex API requires an Authorization header, add it if necessary
	// req.Header.Set("Authorization", "Bearer YOUR_TELEX_API_TOKEN")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send message to Telex: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Telex API returned non-2xx status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// GetChannelMessages fetches historical messages for a given channel.
// This is a placeholder and assumes a Telex API endpoint for fetching channel history.
// You might need to adjust the URL, authentication, and response parsing
// based on the actual Telex API documentation.
func (c *Client) GetChannelMessages(ctx context.Context, channelID string) ([]Message, error) {
	url := fmt.Sprintf("%s/channels/%s/messages", c.baseURL, channelID) // Example URL

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request for historical messages: %w", err)
	}

	// Assuming Telex API requires an Authorization header, add it if necessary
	// req.Header.Set("Authorization", "Bearer YOUR_TELEX_API_TOKEN")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch historical messages from Telex: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Telex API returned non-200 status for historical messages: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var messages []Message // Assuming the API returns a JSON array of messages
	if err := json.NewDecoder(resp.Body).Decode(&messages); err != nil {
		return nil, fmt.Errorf("failed to decode historical messages response: %w", err)
	}

	return messages, nil
}
