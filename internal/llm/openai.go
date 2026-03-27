package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type OpenAIClient struct {
	apiKey string
	model  string
}

// NewOpenAIClient creates a new OpenAI client
func NewOpenAIClient(apiKey, model string) (*OpenAIClient, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required")
	}
	if model == "" {
		model = "gpt-5.4-mini"
	}
	return &OpenAIClient{apiKey: apiKey, model: model}, nil
}

// GenerateCommitMessage generates a commit message using OpenAI
func (c *OpenAIClient) GenerateCommitMessage(diff string) (*CommitMessage, error) {
	schema := jsonSchemaObject{
		Type: "object",
		Properties: map[string]jsonSchemaProperty{
			"header":      {Type: "string", Description: "Short imperative commit header, 50 chars max"},
			"description": {Type: "string", Description: "Longer explanation of what changed and why"},
		},
		Required:             []string{"header", "description"},
		AdditionalProperties: false,
	}

	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal schema: %w", err)
	}

	reqBody := openAIRequest{
		Model: c.model,
		Input: []openAIInput{
			{
				Role: "system",
				Content: []openAIContentBlock{
					{Type: "input_text", Text: "You write concise, well-structured git commit messages."},
				},
			},
			{
				Role: "user",
				Content: []openAIContentBlock{
					{Type: "input_text", Text: fmt.Sprintf(commitPrompt, diff)},
				},
			},
		},
		Text: openAITextConfig{
			Format: openAIFormat{
				Type:   "json_schema",
				Name:   "commit_message",
				Strict: true,
				Schema: schemaBytes,
			},
		},
		MaxOutputTokens: 1000,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/responses", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

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
		var errorResp openAIErrorResponse
		if err := json.Unmarshal(body, &errorResp); err == nil && errorResp.Error.Message != "" {
			return nil, fmt.Errorf("OpenAI API error: %s", errorResp.Error.Message)
		}
		return nil, fmt.Errorf("OpenAI API error: status %d - %s", resp.StatusCode, string(body))
	}

	var openAIResp openAIResponse
	if err := json.Unmarshal(body, &openAIResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if openAIResp.Status != "completed" {
		return nil, fmt.Errorf("OpenAI response status: %s", openAIResp.Status)
	}

	if len(openAIResp.Output) > 0 && len(openAIResp.Output[0].Content) > 0 {
		first := openAIResp.Output[0].Content[0]
		if first.Type == "refusal" {
			return nil, fmt.Errorf("OpenAI refused the request: %s", first.Refusal)
		}
	}

	if openAIResp.OutputText == "" {
		return nil, fmt.Errorf("empty response from OpenAI")
	}

	return parseJSONCommitMessage(openAIResp.OutputText)
}

// --- Request types ---

type openAIRequest struct {
	Model           string           `json:"model"`
	Input           []openAIInput    `json:"input"`
	Text            openAITextConfig `json:"text"`
	MaxOutputTokens int              `json:"max_output_tokens"`
}

type openAIInput struct {
	Role    string               `json:"role"`
	Content []openAIContentBlock `json:"content"`
}

type openAIContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type openAITextConfig struct {
	Format openAIFormat `json:"format"`
}

type openAIFormat struct {
	Type   string          `json:"type"`
	Name   string          `json:"name"`
	Strict bool            `json:"strict"`
	Schema json.RawMessage `json:"schema"`
}

// --- Response types ---

type openAIResponse struct {
	ID         string         `json:"id"`
	Status     string         `json:"status"`
	Output     []openAIOutput `json:"output"`
	OutputText string         `json:"output_text"`
}

type openAIOutput struct {
	Content []openAIOutputContent `json:"content"`
}

type openAIOutputContent struct {
	Type    string `json:"type"`
	Text    string `json:"text,omitempty"`
	Refusal string `json:"refusal,omitempty"`
}

type openAIErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}
