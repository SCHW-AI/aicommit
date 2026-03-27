package ui_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/SCHW-AI/aicommit/internal/config"
	"github.com/SCHW-AI/aicommit/internal/provider"
	"github.com/SCHW-AI/aicommit/internal/ui"
)

type memoryStore struct {
	values map[provider.Provider]string
}

func newMemoryStore() *memoryStore {
	return &memoryStore{values: map[provider.Provider]string{}}
}

func (m *memoryStore) Get(p provider.Provider) (string, error) {
	value, ok := m.values[p]
	if !ok {
		return "", config.ErrSecretNotFound
	}
	return value, nil
}

func (m *memoryStore) Set(p provider.Provider, value string) error {
	m.values[p] = value
	return nil
}

func (m *memoryStore) Delete(p provider.Provider) error {
	delete(m.values, p)
	return nil
}

func (m *memoryStore) Exists(p provider.Provider) (bool, error) {
	_, ok := m.values[p]
	return ok, nil
}

func newHandler(t *testing.T) (http.Handler, *memoryStore, *bool) {
	t.Helper()
	store := newMemoryStore()
	manager, err := config.NewManagerWithStore(filepath.Join(t.TempDir(), "config.yaml"), store)
	if err != nil {
		t.Fatalf("NewManagerWithStore failed: %v", err)
	}
	shutdownCalled := false
	return ui.NewHandler(manager, func() { shutdownCalled = true }), store, &shutdownCalled
}

func TestStateEndpoint(t *testing.T) {
	handler, _, _ := newHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/state", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"anthropic"`)) {
		t.Fatalf("expected anthropic provider in payload: %s", rec.Body.String())
	}
}

func TestConfigAndKeyEndpoints(t *testing.T) {
	handler, store, shutdownCalled := newHandler(t)

	post := func(path string, payload any) *httptest.ResponseRecorder {
		body, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	rec := post("/api/config", map[string]any{
		"provider":        "anthropic",
		"model":           "claude-haiku-4-5-20251001",
		"max_diff_length": 31000,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("config save failed: %s", rec.Body.String())
	}

	rec = post("/api/key", map[string]any{
		"provider": "anthropic",
		"key":      "secret",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("key save failed: %s", rec.Body.String())
	}
	if store.values[provider.Anthropic] != "secret" {
		t.Fatalf("expected secret to be stored, got %#v", store.values)
	}

	rec = post("/api/save-and-validate", map[string]any{
		"provider":        "anthropic",
		"model":           "claude-haiku-4-5-20251001",
		"max_diff_length": 31000,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("save-and-validate failed: %s", rec.Body.String())
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/key?provider=anthropic", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete key failed: %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/shutdown", bytes.NewReader([]byte(`{}`)))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !*shutdownCalled {
		t.Fatalf("expected shutdown to be called, status=%d called=%t", rec.Code, *shutdownCalled)
	}
}
