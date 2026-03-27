package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type GeminiClient struct {
	apiKey string
	model  string
}

// NewGeminiClient creates a new Gemini client
func NewGeminiClient(apiKey, model string) (*GeminiClient, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("Gemini API key is required")
	}
	if model == "" {
		model = "gemini-3-flash-preview"
	}
	if !strings.HasPrefix(model, "models/") {
		model = "models/" + model
	}
	return &GeminiClient{apiKey: apiKey, model: model}, nil
}

// GenerateCommitMessage generates a commit message using Gemini
func (c *GeminiClient) GenerateCommitMessage(diff string) (*CommitMessage, error) {
	reqBody := geminiRequest{
		Contents: []geminiContent{
			{
				Parts: []geminiPart{
					{Text: fmt.Sprintf(commitPrompt, diff)},
				},
			},
		},
		GenerationConfig: geminiGenConfig{
			ResponseMimeType: "application/json",
			ResponseSchema: geminiSchema{
				Type: "OBJECT",
				Properties: map[string]geminiSchemaProperty{
					"header":      {Type: "STRING"},
					"description": {Type: "STRING"},
				},
				Required:         []string{"header", "description"},
				PropertyOrdering: []string{"header", "description"},
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	apiURL := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/%s:generateContent", c.model)

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", c.apiKey)

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
		var errorResp geminiErrorResponse
		if err := json.Unmarshal(body, &errorResp); err == nil && errorResp.Error.Message != "" {
			return nil, fmt.Errorf("Gemini API error: %s", errorResp.Error.Message)
		}
		return nil, fmt.Errorf("Gemini API error: status %d - %s", resp.StatusCode, string(body))
	}

	var geminiResp geminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty response from Gemini")
	}

	return parseJSONCommitMessage(geminiResp.Candidates[0].Content.Parts[0].Text)
}

type geminiRequest struct {
	Contents         []geminiContent `json:"contents"`
	GenerationConfig geminiGenConfig `json:"generationConfig"`
}

type geminiGenConfig struct {
	ResponseMimeType string       `json:"response_mime_type"`
	ResponseSchema   geminiSchema `json:"response_schema"`
}

type geminiSchema struct {
	Type             string                          `json:"type"`
	Properties       map[string]geminiSchemaProperty `json:"properties"`
	Required         []string                        `json:"required"`
	PropertyOrdering []string                        `json:"propertyOrdering,omitempty"`
}

type geminiSchemaProperty struct {
	Type string `json:"type"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

type geminiErrorResponse struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}
