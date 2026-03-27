package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type AnthropicClient struct {
	apiKey string
	model  string
}

// NewAnthropicClient creates a new Anthropic client
func NewAnthropicClient(apiKey, model string) (*AnthropicClient, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("Anthropic API key is required")
	}

	// Default model if not specified
	if model == "" || !isClaudeModel(model) {
		model = "claude-haiku-4-5-20251001"
	}

	return &AnthropicClient{
		apiKey: apiKey,
		model:  model,
	}, nil
}

func isClaudeModel(model string) bool {
	validModels := []string{
		// Claude 4.5 family (latest)
		"claude-opus-4-5-20251101",
		"claude-sonnet-4-5-20250929",
		"claude-haiku-4-5-20251001",

		// Claude 4.1 family
		"claude-opus-4-1-20250805",

		// Claude 4 family
		"claude-sonnet-4-20250522",

		// Legacy Claude 3 (still supported but deprecated)
		"claude-3-opus-20240229",
		"claude-3-sonnet-20240229",
		"claude-3-5-sonnet-20240620",
		"claude-3-haiku-20240307",
		"claude-3-5-haiku-20241022",
	}

	for _, valid := range validModels {
		if model == valid {
			return true
		}
	}
	return false
}

// GenerateCommitMessage generates a commit message using Claude
func (c *AnthropicClient) GenerateCommitMessage(diff string) (*CommitMessage, error) {
	// Prepare the request
	reqBody := anthropicRequest{
		Model:     c.model,
		MaxTokens: 1000,
		Messages: []anthropicMessage{
			{
				Role: "user",
				Content: []anthropicContent{
					{
						Type: "text",
						Text: fmt.Sprintf(commitPrompt, diff),
					},
				},
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create the HTTP request
	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errorResp anthropicErrorResponse
		if err := json.Unmarshal(body, &errorResp); err == nil && errorResp.Error.Message != "" {
			return nil, fmt.Errorf("Anthropic API error: %s", errorResp.Error.Message)
		}
		return nil, fmt.Errorf("Anthropic API error: status %d - %s", resp.StatusCode, string(body))
	}

	// Parse the response
	var anthropicResp anthropicResponse
	if err := json.Unmarshal(body, &anthropicResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(anthropicResp.Content) == 0 {
		return nil, fmt.Errorf("empty response from Anthropic")
	}

	// Parse the commit message from the response
	return ParseResponse(anthropicResp.Content[0].Text)
}

// Anthropic API types
type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string             `json:"role"`
	Content []anthropicContent `json:"content"`
}

type anthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Model        string `json:"model"`
	StopReason   string `json:"stop_reason"`
	StopSequence string `json:"stop_sequence"`
	Usage        struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type anthropicErrorResponse struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}
