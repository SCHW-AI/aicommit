package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/SCHW-AI/aicommit/internal/config"
	"github.com/SCHW-AI/aicommit/internal/provider"
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

func newManager(t *testing.T) (*config.Manager, *memoryStore, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	store := newMemoryStore()
	manager, err := config.NewManagerWithStore(path, store)
	if err != nil {
		t.Fatalf("NewManagerWithStore failed: %v", err)
	}
	return manager, store, path
}

func TestLoadMissingConfig(t *testing.T) {
	manager, _, _ := newManager(t)
	_, err := manager.Load()
	if !errors.Is(err, config.ErrConfigNotFound) {
		t.Fatalf("expected ErrConfigNotFound, got %v", err)
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	manager, _, path := newManager(t)
	cfg := config.Config{
		Provider:      provider.Anthropic,
		Model:         provider.DefaultModel(provider.Anthropic),
		MaxDiffLength: 42000,
	}

	if err := manager.Save(cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := manager.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded != cfg {
		t.Fatalf("loaded config mismatch: %#v != %#v", loaded, cfg)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config file not written: %v", err)
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	manager, _, path := newManager(t)
	if err := os.WriteFile(path, []byte("provider: ["), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	_, err := manager.Load()
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
}

func TestRejectInvalidConfigValues(t *testing.T) {
	manager, _, _ := newManager(t)

	err := manager.Save(config.Config{
		Provider:      provider.Anthropic,
		Model:         "gpt-5-mini",
		MaxDiffLength: 30000,
	})
	if err == nil {
		t.Fatal("expected model/provider validation error")
	}

	err = manager.Save(config.Config{
		Provider:      provider.Anthropic,
		Model:         provider.DefaultModel(provider.Anthropic),
		MaxDiffLength: 0,
	})
	if err == nil {
		t.Fatal("expected diff length validation error")
	}
}

func TestSummaryAndSecrets(t *testing.T) {
	manager, store, _ := newManager(t)
	cfg := config.DefaultConfig()
	if err := manager.Save(cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	if err := manager.SetKey(string(provider.Anthropic), "secret-1"); err != nil {
		t.Fatalf("SetKey failed: %v", err)
	}

	summary, err := manager.Summary()
	if err != nil {
		t.Fatalf("Summary failed: %v", err)
	}

	if !summary.KeyPresence[provider.Anthropic] {
		t.Fatal("expected anthropic key to be present")
	}
	if summary.KeyPresence[provider.Gemini] {
		t.Fatal("did not expect gemini key to be present")
	}

	apiKey, err := manager.ActiveAPIKey(cfg)
	if err != nil {
		t.Fatalf("ActiveAPIKey failed: %v", err)
	}
	if apiKey != "secret-1" {
		t.Fatalf("unexpected active key %q", apiKey)
	}

	if err := manager.DeleteKey(string(provider.Anthropic)); err != nil {
		t.Fatalf("DeleteKey failed: %v", err)
	}
	if _, err := manager.ActiveAPIKey(cfg); err == nil {
		t.Fatal("expected missing secret error after delete")
	}

	if len(store.values) != 0 {
		t.Fatalf("expected store to be empty, got %#v", store.values)
	}
}
