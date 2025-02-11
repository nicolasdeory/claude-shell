package claude

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

const (
	BaseURL = "https://api.anthropic.com/v1/messages"
)

type Client struct {
	apiKey     string
	httpClient *http.Client
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type CreateMessageRequest struct {
	Model     string    `json:"model"`
	Messages  []Message `json:"messages"`
	MaxTokens int       `json:"max_tokens"`
	System    string    `json:"system,omitempty"`
}

type CreateMessageResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	Role string `json:"role"`
}

func NewClient() *Client {
	return &Client{
		apiKey:     os.Getenv("CLAUDE_API_KEY"),
		httpClient: &http.Client{},
	}
}

func (c *Client) CreateMessage(messages []Message) (string, error) {
	// Filter out system messages and use the last one as system parameter
	var systemMsg string
	var filteredMsgs []Message
	for _, msg := range messages {
		if msg.Role == "system" {
			systemMsg = msg.Content
		} else {
			filteredMsgs = append(filteredMsgs, msg)
		}
	}

	reqBody := CreateMessageRequest{
		Model:     "claude-3-sonnet-20240229",
		Messages:  filteredMsgs,
		MaxTokens: 1000,
		System:    systemMsg,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("error marshaling request: %w", err)
	}

	req, err := http.NewRequest("POST", BaseURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var response CreateMessageResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("error unmarshaling response: %w", err)
	}

	if len(response.Content) == 0 {
		return "", fmt.Errorf("no content in response")
	}

	return response.Content[0].Text, nil
}
