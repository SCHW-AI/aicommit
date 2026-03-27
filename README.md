# AICommit

AICommit is a Go CLI that reads your git diff, asks an LLM for a commit message, lets you review or edit it, then commits the result. The project is now Go-only, with secure OS-backed secret storage and a browser-based configuration UI.

If you already have the old PowerShell module installed, follow [MIGRATION.md](MIGRATION.md).

## What changed

- PowerShell support is removed from this codebase
- Environment variables are no longer used for config or secrets
- API keys are stored only in your operating system's credential manager
- Anthropic with `claude-haiku-4-5-20251001` is the default out-of-box path

## Install

Download a release binary from GitHub Releases or build from source.

```bash
git clone https://github.com/SCHW-AI/aicommit.git
cd aicommit
./scripts/build.sh
```

On Windows, prefer:

```powershell
.\scripts\build.ps1
```

These scripts use `go env GOEXE` so the local build gets the correct executable suffix for the current platform.

## First-time setup

The recommended setup flow is the configuration UI:

```bash
aicommit config ui
```

This opens a local browser-based settings screen where you can:

- choose a provider
- choose a provider-valid model
- set the diff limit
- securely store or delete API keys

You can also manage settings from the CLI:

```bash
aicommit config show
aicommit config set provider anthropic
aicommit config set model claude-haiku-4-5-20251001
aicommit config set max-diff-length 30000
aicommit config set-key anthropic
aicommit config delete-key anthropic
```

## Usage

```bash
aicommit
aicommit --push
aicommit --clasp
aicommit --wrangler
aicommit --export
```

Behavior:

1. Checks that you are in a git repository
2. Reads tracked and untracked changes
3. Generates a `HEADER` and `DESCRIPTION`
4. Lets you accept, edit, or cancel
5. Stages everything and commits
6. Optionally pushes or deploys

`--export` writes `git-diff-export.txt` and exits without calling the model or mutating git state.

## Configuration model

AICommit stores non-secret settings in a YAML config file and secrets in the OS keychain / credential manager.

Config schema:

```yaml
provider: anthropic
model: claude-haiku-4-5-20251001
max_diff_length: 30000
```

If config is missing or no API key is stored for the active provider, AICommit fails loudly and tells you to run:

```bash
aicommit config ui
```

## Supported providers

- Anthropic
- Gemini
- OpenAI

Default:

- provider: `anthropic`
- model: `claude-haiku-4-5-20251001`

## Development

```bash
go test ./...
go build ./...
```

The repository ships a minimal CI flow and a tagged release flow for GitHub Releases. Package-manager automation is intentionally out of scope until it is fully maintained and tested.
