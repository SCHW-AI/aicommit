package cmd_test

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SCHW-AI/aicommit/cmd"
	"github.com/SCHW-AI/aicommit/internal/config"
	"github.com/SCHW-AI/aicommit/internal/llm"
	"github.com/SCHW-AI/aicommit/internal/provider"
)

type fakeClient struct{}

func (fakeClient) GenerateCommitMessage(string) (*llm.CommitMessage, error) {
	return &llm.CommitMessage{Header: "Test header", Description: "Test body"}, nil
}

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

func createRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, string(output))
		}
	}
	run("init")
	run("config", "user.name", "AICommit Test")
	run("config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("hello\n"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	run("add", ".")
	run("commit", "-m", "initial")
	return dir
}

func TestLegacyPowerShellFlagRejected(t *testing.T) {
	root := cmd.NewRootCmd(cmd.Dependencies{})
	root.SetArgs([]string{"-push"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected legacy flag parse error")
	}
	if !strings.Contains(err.Error(), "unknown shorthand flag") && !strings.Contains(err.Error(), "unknown flag") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMissingConfigFailsLoudly(t *testing.T) {
	repo := createRepo(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer os.Chdir(cwd)
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	store := newMemoryStore()
	managerFactory := func(_ string) (*config.Manager, error) {
		return config.NewManagerWithStore(filepath.Join(t.TempDir(), "missing.yaml"), store)
	}

	root := cmd.NewRootCmd(cmd.Dependencies{
		NewManager: managerFactory,
		NewClient: func(provider.Provider, string, string) (llm.Client, error) {
			return fakeClient{}, nil
		},
		LaunchConfigUI: func(*config.Manager, io.Writer) error { return nil },
	})
	root.SetArgs([]string{"--export"})
	err = root.Execute()
	if err == nil {
		t.Fatal("expected missing config error")
	}
	if !strings.Contains(err.Error(), "config ui") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfigShowReportsPresenceOnly(t *testing.T) {
	store := newMemoryStore()
	managerFactory := func(_ string) (*config.Manager, error) {
		manager, err := config.NewManagerWithStore(filepath.Join(t.TempDir(), "config.yaml"), store)
		if err != nil {
			return nil, err
		}
		if err := manager.Save(config.DefaultConfig()); err != nil {
			return nil, err
		}
		if err := manager.SetKey(string(provider.Anthropic), "super-secret"); err != nil {
			return nil, err
		}
		return manager, nil
	}

	root := cmd.NewRootCmd(cmd.Dependencies{
		NewManager: managerFactory,
		NewClient: func(provider.Provider, string, string) (llm.Client, error) {
			return fakeClient{}, nil
		},
		LaunchConfigUI: func(*config.Manager, io.Writer) error { return errors.New("not used") },
	})
	buffer := new(strings.Builder)
	root.SetOut(buffer)
	root.SetArgs([]string{"config", "show"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if strings.Contains(buffer.String(), "super-secret") {
		t.Fatalf("config show leaked a secret: %s", buffer.String())
	}
	if !strings.Contains(buffer.String(), "Key stored for anthropic: true") {
		t.Fatalf("expected key presence in output: %s", buffer.String())
	}
}
