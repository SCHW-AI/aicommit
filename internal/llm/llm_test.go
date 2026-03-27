package llm

import (
	"strings"
	"testing"

	"github.com/SCHW-AI/aicommit/internal/provider"
)

func TestParseResponseValid(t *testing.T) {
	msg, err := ParseResponse("HEADER: Add config UI\nDESCRIPTION: Adds a browser-based settings experience")
	if err != nil {
		t.Fatalf("ParseResponse failed: %v", err)
	}
	if msg.Header != "Add config UI" {
		t.Fatalf("unexpected header %q", msg.Header)
	}
	if msg.Description == "" {
		t.Fatal("expected description")
	}
}

func TestParseResponseRequiresHeader(t *testing.T) {
	_, err := ParseResponse("DESCRIPTION: only body")
	if err == nil {
		t.Fatal("expected missing header error")
	}
}

func TestParseResponseAllowsLongHeader(t *testing.T) {
	msg, err := ParseResponse("HEADER: " + strings.Repeat("a", 51))
	if err != nil {
		t.Fatalf("ParseResponse failed: %v", err)
	}
	if len(msg.Header) != 51 {
		t.Fatalf("expected 51-char header, got %d", len(msg.Header))
	}
}

func TestNewClientValidatesProviderModelPair(t *testing.T) {
	_, err := NewClient(provider.Anthropic, "gemini-3-flash-preview", "secret")
	if err == nil {
		t.Fatal("expected provider/model mismatch")
	}
}
