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
		"claude-haiku-4-5-20251001",
		"claude-sonnet-4-5-20250929",
		"claude-opus-4-5-20251101",
		"claude-opus-4-1-20250805",
		"claude-sonnet-4-20250522",
		"claude-3-opus-20240229",
		"claude-3-sonnet-20240229",
		"claude-3-5-sonnet-20240620",
		"claude-3-haiku-20240307",
		"claude-3-5-haiku-20241022",
	},
	Gemini: {
		"gemini-2.5-flash",
		"gemini-2.5-flash-lite",
		"gemini-2.5-pro",
		"gemini-2.0-flash",
		"gemini-1.5-pro",
		"gemini-1.5-flash",
		"gemini-pro",
		"gemini-pro-vision",
	},
	OpenAI: {
		"gpt-5-mini",
		"gpt-5",
		"gpt-5.1",
		"gpt-5-nano",
		"gpt-4.1",
		"gpt-4.1-mini",
		"gpt-4.1-nano",
		"gpt-4",
		"gpt-4-turbo",
		"gpt-4-turbo-preview",
		"gpt-4-0125-preview",
		"gpt-4-1106-preview",
		"gpt-3.5-turbo",
		"gpt-3.5-turbo-0125",
		"gpt-3.5-turbo-1106",
	},
}

var defaults = map[Provider]string{
	Anthropic: "claude-haiku-4-5-20251001",
	Gemini:    "gemini-2.5-flash",
	OpenAI:    "gpt-5-mini",
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
