package llm

import (
	"fmt"
	"strings"

	"github.com/SCHW-AI/aicommit/internal/provider"
)

// CommitMessage represents a structured commit message
type CommitMessage struct {
	Header      string
	Description string
}

// Format returns the formatted commit message
func (c *CommitMessage) Format() string {
	if c.Description == "" {
		return c.Header
	}
	return fmt.Sprintf("%s\n\n%s", c.Header, c.Description)
}

// Client interface for LLM providers
type Client interface {
	GenerateCommitMessage(diff string) (*CommitMessage, error)
}

// NewClient creates a new LLM client for the selected provider and model.
func NewClient(providerValue provider.Provider, model, apiKey string) (Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required for provider %q", providerValue)
	}
	if model == "" {
		model = provider.DefaultModel(providerValue)
	}
	if err := provider.ValidateModel(providerValue, model); err != nil {
		return nil, err
	}

	switch providerValue {
	case provider.Anthropic:
		return NewAnthropicClient(apiKey, model)
	case provider.Gemini:
		return NewGeminiClient(apiKey, model)
	case provider.OpenAI:
		return NewOpenAIClient(apiKey, model)
	default:
		return nil, fmt.Errorf("unsupported provider %q", providerValue)
	}
}

// Common prompt for all providers
const commitPrompt = `Analyze this git diff and suggest a commit message. 

CRITICAL: You must respond in EXACTLY this format. Do not add any other text, explanations, or formatting:

HEADER: [your header text here]
DESCRIPTION: [your description text here]

STRICT REQUIREMENTS:
- Start with exactly "HEADER: " (including the space after colon)
- Header must be 50 characters or less
- Use imperative mood (Add, Fix, Update - NOT Added, Fixed, Updated)
- Then a blank line
- Then start with exactly "DESCRIPTION: " (including the space after colon)
- Description should explain what changed and why
- Do not use markdown, bullets, or special formatting
- Do not add introductory text like "Here's a suggested commit message"
- Do not add closing text or explanations
- Your response should contain ONLY these two lines

EXAMPLE FORMAT:
HEADER: Add user authentication system
DESCRIPTION: Implements login/logout functionality with JWT tokens and password hashing for secure user management

Now analyze this diff:

%s`

// ParseResponse parses the LLM response into a CommitMessage
func ParseResponse(response string) (*CommitMessage, error) {
	lines := strings.Split(response, "\n")

	var header, description string

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "HEADER:") {
			header = strings.TrimSpace(strings.TrimPrefix(line, "HEADER:"))
		} else if strings.HasPrefix(line, "DESCRIPTION:") {
			description = strings.TrimSpace(strings.TrimPrefix(line, "DESCRIPTION:"))
		}
	}

	if header == "" {
		return nil, fmt.Errorf("no header found in response")
	}

	if len(header) > 50 {
		return nil, fmt.Errorf("header exceeds 50 characters")
	}

	return &CommitMessage{
		Header:      header,
		Description: description,
	}, nil
}

// TruncateDiff truncates the diff if it's too long
func TruncateDiff(diff string, maxLength int) string {
	if len(diff) <= maxLength {
		return diff
	}
	return diff[:maxLength] + "\n... (diff truncated)"
}
