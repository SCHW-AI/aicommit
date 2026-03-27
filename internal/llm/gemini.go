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

	// Default model if not specified
	if model == "" || !isGeminiModel(model) {
		model = "gemini-2.5-flash"
	}

	// Ensure model has proper prefix
	if !strings.HasPrefix(model, "models/") {
		model = "models/" + model
	}

	return &GeminiClient{
		apiKey: apiKey,
		model:  model,
	}, nil
}

func isGeminiModel(model string) bool {
	// Remove "models/" prefix if present
	model = strings.TrimPrefix(model, "models/")

	validModels := []string{
		// Gemini 2.5 family (latest stable)
		"gemini-2.5-pro",
		"gemini-2.5-flash",
		"gemini-2.5-flash-lite",

		// Gemini 2.0 family
		"gemini-2.0-flash",

		// Legacy Gemini 1.5 (deprecated)
		"gemini-1.5-pro",
		"gemini-1.5-flash",
		"gemini-pro",
		"gemini-pro-vision",
	}

	for _, valid := range validModels {
		if model == valid {
			return true
		}
	}
	return false
}

// GenerateCommitMessage generates a commit message using Gemini
func (c *GeminiClient) GenerateCommitMessage(diff string) (*CommitMessage, error) {
	// Prepare the request
	reqBody := geminiRequest{
		Contents: []geminiContent{
			{
				Parts: []geminiPart{
					{
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

	// Create the API URL
	apiURL := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/%s:generateContent", c.model)

	// Create the HTTP request
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", c.apiKey)

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
		var errorResp geminiErrorResponse
		if err := json.Unmarshal(body, &errorResp); err == nil && errorResp.Error.Message != "" {
			return nil, fmt.Errorf("Gemini API error: %s", errorResp.Error.Message)
		}
		return nil, fmt.Errorf("Gemini API error: status %d - %s", resp.StatusCode, string(body))
	}

	// Parse the response
	var geminiResp geminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty response from Gemini")
	}

	// Parse the commit message from the response
	return ParseResponse(geminiResp.Candidates[0].Content.Parts[0].Text)
}

// Gemini API types
type geminiRequest struct {
	Contents []geminiContent `json:"contents"`
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
			Role string `json:"role"`
		} `json:"content"`
		FinishReason  string `json:"finishReason"`
		Index         int    `json:"index"`
		SafetyRatings []struct {
			Category    string `json:"category"`
			Probability string `json:"probability"`
		} `json:"safetyRatings"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}

type geminiErrorResponse struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}
