package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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

	// Default model if not specified
	if model == "" || !isOpenAIModel(model) {
		model = "gpt-5-mini"
	}

	return &OpenAIClient{
		apiKey: apiKey,
		model:  model,
	}, nil
}

func isOpenAIModel(model string) bool {
	validModels := []string{
		// GPT-5 family (latest)
		"gpt-5.1",
		"gpt-5",
		"gpt-5-mini",
		"gpt-5-nano",

		// GPT-4.1 family
		"gpt-4.1",
		"gpt-4.1-mini",
		"gpt-4.1-nano",

		// GPT-4 family (legacy)
		"gpt-4",
		"gpt-4-turbo",
		"gpt-4-turbo-preview",
		"gpt-4-0125-preview",
		"gpt-4-1106-preview",
		"gpt-3.5-turbo",
		"gpt-3.5-turbo-0125",
		"gpt-3.5-turbo-1106",
	}

	for _, valid := range validModels {
		if strings.HasPrefix(model, valid) {
			return true
		}
	}
	return false
}

// GenerateCommitMessage generates a commit message using OpenAI
func (c *OpenAIClient) GenerateCommitMessage(diff string) (*CommitMessage, error) {
	// Prepare the request
	reqBody := openAIRequest{
		Model: c.model,
		Messages: []openAIMessage{
			{
				Role:    "system",
				Content: "You are a helpful assistant that generates concise, well-structured git commit messages.",
			},
			{
				Role:    "user",
				Content: fmt.Sprintf(commitPrompt, diff),
			},
		},
		Temperature: 0.3,
		MaxTokens:   1000,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create the HTTP request
	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

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
		var errorResp openAIErrorResponse
		if err := json.Unmarshal(body, &errorResp); err == nil && errorResp.Error.Message != "" {
			return nil, fmt.Errorf("OpenAI API error: %s", errorResp.Error.Message)
		}
		return nil, fmt.Errorf("OpenAI API error: status %d - %s", resp.StatusCode, string(body))
	}

	// Parse the response
	var openAIResp openAIResponse
	if err := json.Unmarshal(body, &openAIResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(openAIResp.Choices) == 0 {
		return nil, fmt.Errorf("empty response from OpenAI")
	}

	// Parse the commit message from the response
	return ParseResponse(openAIResp.Choices[0].Message.Content)
}

// OpenAI API types
type openAIRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Temperature float64         `json:"temperature"`
	MaxTokens   int             `json:"max_tokens"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int    `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type openAIErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Param   string `json:"param"`
		Code    string `json:"code"`
	} `json:"error"`
}
