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
	if model == "" {
		model = "claude-haiku-4-5-20251001"
	}
	return &AnthropicClient{apiKey: apiKey, model: model}, nil
}

// GenerateCommitMessage generates a commit message using Claude
func (c *AnthropicClient) GenerateCommitMessage(diff string) (*CommitMessage, error) {
	reqBody := anthropicRequest{
		Model:     c.model,
		MaxTokens: 1000,
		Messages: []anthropicMessage{
			{Role: "user", Content: fmt.Sprintf(commitPrompt, diff)},
		},
		Tools: []anthropicTool{
			{
				Name:        "emit_commit_message",
				Description: "Return the commit message as structured JSON.",
				Strict:      true,
				InputSchema: jsonSchemaObject{
					Type: "object",
					Properties: map[string]jsonSchemaProperty{
						"header":      {Type: "string", Description: "Short imperative commit header, 50 chars max"},
						"description": {Type: "string", Description: "Longer explanation of what changed and why"},
					},
					Required:             []string{"header", "description"},
					AdditionalProperties: false,
				},
			},
		},
		ToolChoice: anthropicToolChoice{
			Type:                   "tool",
			Name:                   "emit_commit_message",
			DisableParallelToolUse: true,
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

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

	var anthropicResp anthropicResponse
	if err := json.Unmarshal(body, &anthropicResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	for _, block := range anthropicResp.Content {
		if block.Type == "tool_use" && block.Name == "emit_commit_message" {
			var msg CommitMessage
			if err := json.Unmarshal(block.Input, &msg); err != nil {
				return nil, fmt.Errorf("failed to parse tool_use input: %w", err)
			}
			if msg.Header == "" {
				return nil, fmt.Errorf("empty header in tool_use response")
			}
			return &msg, nil
		}
	}

	return nil, fmt.Errorf("no tool_use block found in Anthropic response")
}

// --- Request types ---

type anthropicRequest struct {
	Model      string              `json:"model"`
	MaxTokens  int                 `json:"max_tokens"`
	Messages   []anthropicMessage  `json:"messages"`
	Tools      []anthropicTool     `json:"tools"`
	ToolChoice anthropicToolChoice `json:"tool_choice"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicTool struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Strict      bool             `json:"strict"`
	InputSchema jsonSchemaObject `json:"input_schema"`
}

type anthropicToolChoice struct {
	Type                   string `json:"type"`
	Name                   string `json:"name"`
	DisableParallelToolUse bool   `json:"disable_parallel_tool_use"`
}

// --- Shared schema helpers (also used by OpenAI) ---

type jsonSchemaObject struct {
	Type                 string                        `json:"type"`
	Properties           map[string]jsonSchemaProperty `json:"properties"`
	Required             []string                      `json:"required"`
	AdditionalProperties bool                          `json:"additionalProperties"`
}

type jsonSchemaProperty struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

// --- Response types ---

type anthropicResponse struct {
	ID         string                  `json:"id"`
	Type       string                  `json:"type"`
	Role       string                  `json:"role"`
	Content    []anthropicContentBlock `json:"content"`
	StopReason string                  `json:"stop_reason"`
}

type anthropicContentBlock struct {
	Type  string          `json:"type"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
	Text  string          `json:"text,omitempty"`
}

type anthropicErrorResponse struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}
