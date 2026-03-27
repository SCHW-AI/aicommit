package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/99designs/keyring"
	"github.com/adrg/xdg"
	"go.yaml.in/yaml/v3"

	"github.com/SCHW-AI/aicommit/internal/provider"
)

const (
	serviceName = "aicommit"
)

var (
	ErrConfigNotFound = errors.New("config file not found")
	ErrSecretNotFound = errors.New("secret not found")
)

type Config struct {
	Provider      provider.Provider `yaml:"provider"`
	Model         string            `yaml:"model"`
	MaxDiffLength int               `yaml:"max_diff_length"`
}

type Summary struct {
	ConfigPath    string                     `json:"config_path"`
	Provider      provider.Provider          `json:"provider"`
	Model         string                     `json:"model"`
	MaxDiffLength int                        `json:"max_diff_length"`
	KeyPresence   map[provider.Provider]bool `json:"key_presence"`
	ConfigExists  bool                       `json:"config_exists"`
}

type SecretStore interface {
	Get(provider.Provider) (string, error)
	Set(provider.Provider, string) error
	Delete(provider.Provider) error
	Exists(provider.Provider) (bool, error)
}

type Manager struct {
	path  string
	store SecretStore
}

type keyringStore struct {
	ring keyring.Keyring
}

func DefaultConfig() Config {
	return Config{
		Provider:      provider.Anthropic,
		Model:         provider.DefaultModel(provider.Anthropic),
		MaxDiffLength: provider.DefaultMaxDiffLength,
	}
}

func DefaultPath() (string, error) {
	return filepath.Join(xdg.ConfigHome, "aicommit", "config.yaml"), nil
}

func NewManager(configPath string) (*Manager, error) {
	store, err := newKeyringStore()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize secure credential storage: %w", err)
	}
	return NewManagerWithStore(configPath, store)
}

func NewManagerWithStore(configPath string, store SecretStore) (*Manager, error) {
	if store == nil {
		return nil, fmt.Errorf("secret store is required")
	}
	if configPath == "" {
		var err error
		configPath, err = DefaultPath()
		if err != nil {
			return nil, err
		}
	}
	return &Manager{
		path:  configPath,
		store: store,
	}, nil
}

func newKeyringStore() (*keyringStore, error) {
	ring, err := keyring.Open(keyring.Config{
		ServiceName: serviceName,
		AllowedBackends: []keyring.BackendType{
			keyring.KeychainBackend,
			keyring.WinCredBackend,
			keyring.SecretServiceBackend,
		},
	})
	if err != nil {
		return nil, err
	}
	return &keyringStore{ring: ring}, nil
}

func (m *Manager) Path() string {
	return m.path
}

func (m *Manager) Load() (Config, error) {
	data, err := os.ReadFile(m.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, ErrConfigNotFound
		}
		return Config{}, fmt.Errorf("failed to read config: %w", err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("failed to parse config: %w", err)
	}

	if err := Validate(cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (m *Manager) LoadOrDefault() (Config, error) {
	cfg, err := m.Load()
	if errors.Is(err, ErrConfigNotFound) {
		return DefaultConfig(), nil
	}
	return cfg, err
}

func (m *Manager) Save(cfg Config) error {
	if err := Validate(cfg); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(m.path), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to serialize config: %w", err)
	}
	if err := os.WriteFile(m.path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	return nil
}

func Validate(cfg Config) error {
	if _, err := provider.ParseProvider(string(cfg.Provider)); err != nil {
		return err
	}
	if cfg.MaxDiffLength <= 0 {
		return fmt.Errorf("max_diff_length must be greater than zero")
	}
	if err := provider.ValidateModel(cfg.Provider, cfg.Model); err != nil {
		return err
	}
	return nil
}

func (m *Manager) Summary() (Summary, error) {
	cfg, err := m.Load()
	configExists := true
	if errors.Is(err, ErrConfigNotFound) {
		cfg = DefaultConfig()
		configExists = false
	} else if err != nil {
		return Summary{}, err
	}

	keyPresence := make(map[provider.Provider]bool, len(provider.Providers()))
	for _, item := range provider.Providers() {
		exists, err := m.store.Exists(item)
		if err != nil {
			return Summary{}, err
		}
		keyPresence[item] = exists
	}

	return Summary{
		ConfigPath:    m.path,
		Provider:      cfg.Provider,
		Model:         cfg.Model,
		MaxDiffLength: cfg.MaxDiffLength,
		KeyPresence:   keyPresence,
		ConfigExists:  configExists,
	}, nil
}

func (m *Manager) ActiveAPIKey(cfg Config) (string, error) {
	secret, err := m.store.Get(cfg.Provider)
	if err != nil {
		if errors.Is(err, ErrSecretNotFound) {
			return "", fmt.Errorf("no API key stored for provider %q", cfg.Provider)
		}
		return "", err
	}
	if secret == "" {
		return "", fmt.Errorf("no API key stored for provider %q", cfg.Provider)
	}
	return secret, nil
}

func (m *Manager) SetKey(rawProvider, value string) error {
	providerValue, err := provider.ParseProvider(rawProvider)
	if err != nil {
		return err
	}
	if value == "" {
		return fmt.Errorf("API key cannot be empty")
	}
	return m.store.Set(providerValue, value)
}

func (m *Manager) DeleteKey(rawProvider string) error {
	providerValue, err := provider.ParseProvider(rawProvider)
	if err != nil {
		return err
	}
	return m.store.Delete(providerValue)
}

func (m *Manager) HasKey(providerValue provider.Provider) (bool, error) {
	return m.store.Exists(providerValue)
}

func keyName(providerValue provider.Provider) string {
	return fmt.Sprintf("%s_api_key", providerValue)
}

func (s *keyringStore) Get(providerValue provider.Provider) (string, error) {
	item, err := s.ring.Get(keyName(providerValue))
	if err != nil {
		if errors.Is(err, keyring.ErrKeyNotFound) {
			return "", ErrSecretNotFound
		}
		return "", err
	}
	return string(item.Data), nil
}

func (s *keyringStore) Set(providerValue provider.Provider, value string) error {
	return s.ring.Set(keyring.Item{
		Key:  keyName(providerValue),
		Data: []byte(value),
	})
}

func (s *keyringStore) Delete(providerValue provider.Provider) error {
	err := s.ring.Remove(keyName(providerValue))
	if errors.Is(err, keyring.ErrKeyNotFound) {
		return nil
	}
	return err
}

func (s *keyringStore) Exists(providerValue provider.Provider) (bool, error) {
	_, err := s.Get(providerValue)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, ErrSecretNotFound) {
		return false, nil
	}
	return false, err
}
