# Migrating From The PowerShell Module

This guide is for people who already have the old PowerShell-based `AICommit` module installed and want to move to the new Go CLI.

## What changed

- The PowerShell module files are gone from this branch.
- The new app is a standalone Go binary, not an imported PowerShell module.
- API keys are no longer read from environment variables.
- Secrets are now stored in your operating system's secure credential store.
- Command flags now use standard CLI syntax such as `--push` instead of PowerShell-style switches like `-push`.
- The default provider/model changed from Gemini to Anthropic `claude-haiku-4-5-20251001`.

## Quick migration

1. Remove the old module import from your PowerShell profile.
2. Install the new `aicommit` binary and put it on your `PATH`.
3. Run `aicommit config ui` and store your provider, model, diff limit, and API key.
4. Verify that PowerShell is running the new binary instead of the old module command.

## Step 1: Remove the old PowerShell startup lines

If you added AICommit to your PowerShell profile, open it and remove the old AICommit lines:

```powershell
notepad $PROFILE
```

Common lines to remove:

```powershell
Import-Module "C:\path\to\aicommit-powershell\AICommit.psm1"
$env:ANTHROPIC_API_KEY_AICOMMIT = "..."
$env:GEMINI_API_KEY_AICOMMIT = "..."
$env:AI_COMMIT_MODEL = "..."
$env:AI_COMMIT_MAX_DIFF_LENGTH = "..."
```

If you still have much older env vars from an earlier setup, those are not used by the Go CLI either:

```powershell
$env:ANTHROPIC_API_KEY = "..."
$env:GEMINI_API_KEY = "..."
```

Then reload your profile or open a fresh shell:

```powershell
. $PROFILE
```

## Step 2: Unload or remove the old module

To remove the module from the current session:

```powershell
Remove-Module AICommit -ErrorAction SilentlyContinue
```

If you cloned the old module into your PowerShell modules folder, you can also delete that old copy after you have removed the profile import:

```powershell
Remove-Item "$env:USERPROFILE\Documents\WindowsPowerShell\Modules\AICommit" -Recurse -Force
```

## Step 3: Install the new Go CLI

Install the new binary from GitHub Releases or build it from source:

```powershell
git clone https://github.com/SCHW-AI/aicommit.git
cd aicommit
go build -o aicommit .
```

Make sure the built binary is on your `PATH`, or run it from the build directory.

## Step 4: Recreate your settings in the new config system

The recommended path is:

```powershell
aicommit config ui
```

That opens the local configuration UI so you can:

- choose a provider
- choose a provider-valid model
- set the max diff length
- store the API key in Windows Credential Manager

You can do the same work from the CLI:

```powershell
aicommit config set provider anthropic
aicommit config set model claude-haiku-4-5-20251001
aicommit config set max-diff-length 30000
aicommit config set-key anthropic
```

## Old setup vs new setup

| PowerShell version | Go version |
| --- | --- |
| `Import-Module AICommit` | Install `aicommit` binary |
| `$env:ANTHROPIC_API_KEY_AICOMMIT` | `aicommit config set-key anthropic` |
| `$env:GEMINI_API_KEY_AICOMMIT` | `aicommit config set-key gemini` |
| `$env:AI_COMMIT_MODEL` | `aicommit config set model ...` |
| `$env:AI_COMMIT_MAX_DIFF_LENGTH` | `aicommit config set max-diff-length ...` |
| secrets in profile/env vars | secrets in OS credential store |

## Command changes

The main command name stays `aicommit`, but the flags changed:

```powershell
# Old PowerShell module
aicommit -push
aicommit -clasp
aicommit -wrangler
aicommit -export

# New Go CLI
aicommit --push
aicommit --clasp
aicommit --wrangler
aicommit --export
```

## Step 5: Verify that PowerShell is running the new command

This is the most important check on Windows:

```powershell
Get-Command aicommit -All
```

You want PowerShell to resolve to the new executable, not to a leftover function from `AICommit.psm1`.

Then verify the new config is visible:

```powershell
aicommit config show
```

That command prints the active provider/model, the config file path, and whether a key is stored for each provider.

## Behavior differences to expect

- If config is missing, the Go CLI stops and tells you to run `aicommit config ui`.
- If the active provider has no stored key, the Go CLI stops and tells you to add one.
- The Go CLI validates that the selected model belongs to the selected provider.
- The new default path is Anthropic plus `claude-haiku-4-5-20251001`, not Gemini.

## Troubleshooting

If `aicommit` still behaves like the old module:

1. Run `Get-Command aicommit -All`.
2. Remove the `Import-Module AICommit` line from your profile.
3. Run `Remove-Module AICommit`.
4. Open a new PowerShell session.

If your old env vars are still present, that is harmless, but the new CLI ignores them. The fix is to store the key again with `aicommit config ui` or `aicommit config set-key`.
