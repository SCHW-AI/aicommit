package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"github.com/SCHW-AI/aicommit/internal/config"
	"github.com/SCHW-AI/aicommit/internal/llm"
	"github.com/SCHW-AI/aicommit/internal/provider"
)

type configPayload struct {
	Provider      string `json:"provider"`
	Model         string `json:"model"`
	MaxDiffLength int    `json:"max_diff_length"`
}

type keyPayload struct {
	Provider string `json:"provider"`
	Key      string `json:"key"`
}

type statePayload struct {
	Summary config.Summary      `json:"summary"`
	Models  map[string][]string `json:"models"`
}

func LaunchConfigUI(manager *config.Manager, stdout io.Writer) error {
	server := &http.Server{
		ReadHeaderTimeout: 5 * time.Second,
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("failed to start local config UI: %w", err)
	}

	done := make(chan error, 1)
	shutdown := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		go func() {
			done <- server.Shutdown(ctx)
		}()
	}

	server.Handler = NewHandler(manager, shutdown)

	go func() {
		done <- server.Serve(listener)
	}()

	url := "http://" + listener.Addr().String()
	fmt.Fprintf(stdout, "AICommit configuration UI: %s\n", url)
	if err := openBrowser(url); err != nil {
		fmt.Fprintf(stdout, "Could not open a browser automatically: %v\n", err)
		fmt.Fprintln(stdout, "Open the URL above in your browser.")
	}

	err = <-done
	if err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func NewHandler(manager *config.Manager, shutdown func()) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(configPage))
	})

	mux.HandleFunc("/api/state", func(w http.ResponseWriter, _ *http.Request) {
		summary, err := manager.Summary()
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err)
			return
		}

		models := make(map[string][]string, len(provider.Providers()))
		for _, item := range provider.Providers() {
			models[string(item)] = provider.ModelsFor(item)
		}

		writeJSON(w, http.StatusOK, statePayload{
			Summary: summary,
			Models:  models,
		})
	})

	mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSONError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
			return
		}
		payload, err := decodeConfigPayload(r)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err)
			return
		}
		if err := manager.Save(payload); err != nil {
			writeJSONError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": "Configuration saved"})
	})

	mux.HandleFunc("/api/save-and-validate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSONError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
			return
		}
		payload, err := decodeConfigPayload(r)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err)
			return
		}
		if err := manager.Save(payload); err != nil {
			writeJSONError(w, http.StatusBadRequest, err)
			return
		}
		apiKey, err := manager.ActiveAPIKey(payload)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err)
			return
		}
		if _, err := llm.NewClient(payload.Provider, payload.Model, apiKey); err != nil {
			writeJSONError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": "Configuration saved and validated"})
	})

	mux.HandleFunc("/api/key", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			var payload keyPayload
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				writeJSONError(w, http.StatusBadRequest, fmt.Errorf("invalid key payload"))
				return
			}
			if err := manager.SetKey(payload.Provider, payload.Key); err != nil {
				writeJSONError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"message": "API key stored securely"})
		case http.MethodDelete:
			rawProvider := r.URL.Query().Get("provider")
			if rawProvider == "" {
				writeJSONError(w, http.StatusBadRequest, fmt.Errorf("provider is required"))
				return
			}
			if err := manager.DeleteKey(rawProvider); err != nil {
				writeJSONError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"message": "API key deleted"})
		default:
			writeJSONError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		}
	})

	mux.HandleFunc("/api/shutdown", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSONError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": "Shutting down"})
		shutdown()
	})

	return mux
}

func decodeConfigPayload(r *http.Request) (config.Config, error) {
	var payload configPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return config.Config{}, fmt.Errorf("invalid configuration payload")
	}
	providerValue, err := provider.ParseProvider(payload.Provider)
	if err != nil {
		return config.Config{}, err
	}
	cfg := config.Config{
		Provider:      providerValue,
		Model:         payload.Model,
		MaxDiffLength: payload.MaxDiffLength,
	}
	return cfg, nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeJSONError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

const configPage = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>AICommit Settings</title>
  <style>
    :root {
      --bg: #f7f1e5;
      --panel: #fffdf7;
      --ink: #1f1a17;
      --muted: #6b5d53;
      --accent: #0e5a54;
      --accent-soft: #d7efe9;
      --danger: #b43f2e;
      --border: #d7c9bd;
      --shadow: 0 18px 48px rgba(44, 33, 24, 0.14);
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      font-family: "Segoe UI", "Helvetica Neue", Arial, sans-serif;
      background: radial-gradient(circle at top, #fff9ee 0%, var(--bg) 55%, #efe4d8 100%);
      color: var(--ink);
      min-height: 100vh;
    }
    main {
      max-width: 980px;
      margin: 0 auto;
      padding: 40px 20px 60px;
    }
    .shell {
      background: var(--panel);
      border: 1px solid var(--border);
      border-radius: 26px;
      box-shadow: var(--shadow);
      overflow: hidden;
    }
    .hero {
      padding: 32px 36px 20px;
      background: linear-gradient(135deg, rgba(14, 90, 84, 0.12), rgba(188, 117, 57, 0.06));
      border-bottom: 1px solid var(--border);
    }
    .eyebrow {
      display: inline-block;
      padding: 6px 10px;
      border-radius: 999px;
      background: rgba(14, 90, 84, 0.1);
      color: var(--accent);
      font-size: 12px;
      font-weight: 700;
      letter-spacing: 0.08em;
      text-transform: uppercase;
    }
    h1 {
      margin: 14px 0 8px;
      font-size: 36px;
      line-height: 1.05;
    }
    p {
      margin: 0;
      color: var(--muted);
      line-height: 1.6;
    }
    .grid {
      display: grid;
      grid-template-columns: 1.1fr 0.9fr;
      gap: 24px;
      padding: 28px 32px 32px;
    }
    .card {
      border: 1px solid var(--border);
      border-radius: 20px;
      padding: 22px;
      background: rgba(255, 255, 255, 0.84);
    }
    .card h2 {
      margin: 0 0 16px;
      font-size: 20px;
    }
    label {
      display: block;
      margin: 14px 0 6px;
      font-size: 13px;
      font-weight: 700;
      letter-spacing: 0.03em;
      text-transform: uppercase;
      color: var(--muted);
    }
    input, select, button {
      font: inherit;
    }
    input, select {
      width: 100%;
      padding: 13px 14px;
      border-radius: 14px;
      border: 1px solid var(--border);
      background: white;
      color: var(--ink);
    }
    .actions {
      display: flex;
      flex-wrap: wrap;
      gap: 10px;
      margin-top: 22px;
    }
    button {
      border: 0;
      border-radius: 999px;
      padding: 12px 18px;
      cursor: pointer;
      transition: transform 140ms ease, opacity 140ms ease;
    }
    button:hover { transform: translateY(-1px); }
    .primary { background: var(--accent); color: white; }
    .secondary { background: var(--accent-soft); color: var(--accent); }
    .danger { background: rgba(180, 63, 46, 0.12); color: var(--danger); }
    .ghost { background: #efe5da; color: var(--ink); }
    .status-list {
      display: grid;
      gap: 12px;
      margin-top: 14px;
    }
    .status-row {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 12px 14px;
      border-radius: 14px;
      background: #faf3e9;
      border: 1px solid var(--border);
    }
    .pill {
      padding: 6px 10px;
      border-radius: 999px;
      font-size: 12px;
      font-weight: 700;
    }
    .pill.good {
      background: rgba(14, 90, 84, 0.12);
      color: var(--accent);
    }
    .pill.bad {
      background: rgba(180, 63, 46, 0.12);
      color: var(--danger);
    }
    .meta {
      margin-top: 18px;
      padding: 14px;
      border-radius: 14px;
      background: #f6eee5;
      color: var(--muted);
      font-size: 14px;
      line-height: 1.5;
      word-break: break-word;
    }
    #message {
      min-height: 24px;
      margin-top: 18px;
      font-weight: 600;
    }
    #message.error { color: var(--danger); }
    #message.success { color: var(--accent); }
    @media (max-width: 860px) {
      .grid { grid-template-columns: 1fr; padding: 22px; }
      .hero { padding: 26px 24px 18px; }
      h1 { font-size: 30px; }
    }
  </style>
</head>
<body>
  <main>
    <div class="shell">
      <section class="hero">
        <span class="eyebrow">AICommit Config</span>
        <h1>Set up AICommit once, keep it tidy.</h1>
        <p>Choose the provider, lock the model, tune the diff limit, and store API keys securely in your operating system's credential manager.</p>
      </section>
      <section class="grid">
        <div class="card">
          <h2>Runtime settings</h2>
          <label for="provider">Provider</label>
          <select id="provider"></select>

          <label for="model">Model</label>
          <select id="model"></select>

          <label for="diff">Max diff length</label>
          <input id="diff" type="number" min="1" step="1">

          <label for="apikey">API key for selected provider</label>
          <input id="apikey" type="password" placeholder="Paste a key to store it securely">

          <div class="actions">
            <button class="primary" onclick="saveConfig()">Save</button>
            <button class="secondary" onclick="saveAndValidate()">Save + Validate</button>
            <button class="secondary" onclick="storeKey()">Save Key</button>
            <button class="danger" onclick="deleteKey()">Delete Key</button>
            <button class="ghost" onclick="shutdownUI()">Exit</button>
          </div>

          <div id="message"></div>
        </div>

        <div class="card">
          <h2>Stored keys</h2>
          <p>Secrets are never written to the config file. The indicators below show whether a secure key is stored for each provider.</p>
          <div class="status-list" id="status-list"></div>
          <div class="meta" id="meta"></div>
        </div>
      </section>
    </div>
  </main>

  <script>
    let state = null;

    async function request(url, options = {}) {
      const response = await fetch(url, {
        headers: { 'Content-Type': 'application/json' },
        ...options,
      });
      const data = await response.json();
      if (!response.ok) {
        throw new Error(data.error || 'Request failed');
      }
      return data;
    }

    function setMessage(text, kind) {
      const message = document.getElementById('message');
      message.textContent = text;
      message.className = kind || '';
    }

    function populateModels(providerName, currentModel) {
      const select = document.getElementById('model');
      select.innerHTML = '';
      const models = state.models[providerName] || [];
      for (const model of models) {
        const option = document.createElement('option');
        option.value = model;
        option.textContent = model;
        option.selected = model === currentModel;
        select.appendChild(option);
      }
    }

    function renderState() {
      const providerSelect = document.getElementById('provider');
      providerSelect.innerHTML = '';
      Object.keys(state.models).forEach((providerName) => {
        const option = document.createElement('option');
        option.value = providerName;
        option.textContent = providerName;
        option.selected = providerName === state.summary.provider;
        providerSelect.appendChild(option);
      });

      populateModels(state.summary.provider, state.summary.model);
      document.getElementById('diff').value = state.summary.max_diff_length;
      document.getElementById('apikey').value = '';

      const statusList = document.getElementById('status-list');
      statusList.innerHTML = '';
      for (const [providerName, exists] of Object.entries(state.summary.key_presence)) {
        const row = document.createElement('div');
        row.className = 'status-row';
        row.innerHTML = '<strong>' + providerName + '</strong><span class="pill ' + (exists ? 'good' : 'bad') + '">' + (exists ? 'stored' : 'missing') + '</span>';
        statusList.appendChild(row);
      }

      document.getElementById('meta').innerHTML =
        'Config file: <strong>' + state.summary.config_path + '</strong><br>' +
        'Config exists: <strong>' + state.summary.config_exists + '</strong>';
    }

    async function refresh() {
      state = await request('/api/state');
      renderState();
    }

    function currentPayload() {
      return {
        provider: document.getElementById('provider').value,
        model: document.getElementById('model').value,
        max_diff_length: Number(document.getElementById('diff').value),
      };
    }

    async function saveConfig() {
      try {
        const data = await request('/api/config', {
          method: 'POST',
          body: JSON.stringify(currentPayload()),
        });
        setMessage(data.message, 'success');
        await refresh();
      } catch (error) {
        setMessage(error.message, 'error');
      }
    }

    async function saveAndValidate() {
      try {
        const data = await request('/api/save-and-validate', {
          method: 'POST',
          body: JSON.stringify(currentPayload()),
        });
        setMessage(data.message, 'success');
        await refresh();
      } catch (error) {
        setMessage(error.message, 'error');
      }
    }

    async function storeKey() {
      try {
        const keyField = document.getElementById('apikey');
        const key = keyField.value.trim();
        if (!key) {
          throw new Error('Enter an API key before saving');
        }
        const data = await request('/api/key', {
          method: 'POST',
          body: JSON.stringify({
            provider: document.getElementById('provider').value,
            key,
          }),
        });
        keyField.value = '';
        setMessage(data.message, 'success');
        await refresh();
      } catch (error) {
        setMessage(error.message, 'error');
      }
    }

    async function deleteKey() {
      try {
        const providerName = document.getElementById('provider').value;
        const data = await request('/api/key?provider=' + encodeURIComponent(providerName), {
          method: 'DELETE',
        });
        setMessage(data.message, 'success');
        await refresh();
      } catch (error) {
        setMessage(error.message, 'error');
      }
    }

    async function shutdownUI() {
      try {
        await request('/api/shutdown', { method: 'POST' });
      } finally {
        window.close();
      }
    }

    document.getElementById('provider').addEventListener('change', (event) => {
      const providerName = event.target.value;
      const models = state.models[providerName] || [];
      populateModels(providerName, models[0] || '');
    });

    window.addEventListener('beforeunload', () => {
      navigator.sendBeacon('/api/shutdown', new Blob([JSON.stringify({ closed: true })], { type: 'application/json' }));
    });

    refresh().catch((error) => setMessage(error.message, 'error'));
  </script>
</body>
</html>`
