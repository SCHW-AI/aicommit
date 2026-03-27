package provider

import (
	"fmt"
	"slices"
	"strings"
)

type Provider string

const (
	Anthropic Provider = "anthropic"
	Gemini    Provider = "gemini"
	OpenAI    Provider = "openai"
)

const DefaultMaxDiffLength = 30000

var catalog = map[Provider][]string{
	Anthropic: {
		// Claude 4.6 family (latest)
		"claude-opus-4-6",
		"claude-sonnet-4-6",
		// Claude 4.5 family
		"claude-haiku-4-5-20251001",
		"claude-opus-4-5-20251101",
		"claude-sonnet-4-5-20250929",
		// Legacy Claude 4.x
		"claude-opus-4-1-20250805",
		"claude-sonnet-4-20250522",
		// Legacy Claude 3.x (deprecated)
		"claude-3-opus-20240229",
		"claude-3-sonnet-20240229",
		"claude-3-5-sonnet-20240620",
		"claude-3-haiku-20240307",
		"claude-3-5-haiku-20241022",
	},
	Gemini: {
		// Gemini 3.x family (latest)
		"gemini-3.1-pro-preview",
		"gemini-3-flash-preview",
		"gemini-3.1-flash-lite-preview",
		// Legacy 2.5 family
		"gemini-2.5-pro",
		"gemini-2.5-flash",
		"gemini-2.5-flash-lite",
		// Legacy 2.0
		"gemini-2.0-flash",
	},
	OpenAI: {
		// GPT-5.4 family (latest)
		"gpt-5.4",
		"gpt-5.4-mini",
		"gpt-5.4-nano",
		// Legacy GPT-5.x
		"gpt-5",
		"gpt-5.1",
		"gpt-5-mini",
		"gpt-5-nano",
		// Legacy GPT-4.1
		"gpt-4.1",
		"gpt-4.1-mini",
		"gpt-4.1-nano",
	},
}

var defaults = map[Provider]string{
	Anthropic: "claude-haiku-4-5-20251001",
	Gemini:    "gemini-3-flash-preview",
	OpenAI:    "gpt-5.4-mini",
}

func Providers() []Provider {
	return []Provider{Anthropic, Gemini, OpenAI}
}

func ParseProvider(raw string) (Provider, error) {
	provider := Provider(strings.ToLower(strings.TrimSpace(raw)))
	if _, ok := catalog[provider]; !ok {
		return "", fmt.Errorf("unsupported provider %q", raw)
	}
	return provider, nil
}

func ModelsFor(provider Provider) []string {
	models := catalog[provider]
	return append([]string(nil), models...)
}

func DefaultModel(provider Provider) string {
	return defaults[provider]
}

func ValidateModel(provider Provider, model string) error {
	if model == "" {
		return fmt.Errorf("model is required")
	}
	if _, ok := catalog[provider]; !ok {
		return fmt.Errorf("unsupported provider %q", provider)
	}
	if !slices.Contains(catalog[provider], model) {
		return fmt.Errorf("model %q is not valid for provider %q", model, provider)
	}
	return nil
}
