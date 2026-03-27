package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/SCHW-AI/aicommit/internal/config"
	"github.com/SCHW-AI/aicommit/internal/git"
	"github.com/SCHW-AI/aicommit/internal/llm"
	"github.com/SCHW-AI/aicommit/internal/prompt"
	"github.com/SCHW-AI/aicommit/internal/provider"
	"github.com/SCHW-AI/aicommit/internal/ui"
)

type Dependencies struct {
	NewManager     func(string) (*config.Manager, error)
	NewClient      func(provider.Provider, string, string) (llm.Client, error)
	LaunchConfigUI func(*config.Manager, io.Writer) error
}

var version = "1.0.0"

type rootOptions struct {
	ConfigPath string
	Push       bool
	Clasp      bool
	Wrangler   bool
	Export     bool
}

func defaultDependencies() Dependencies {
	return Dependencies{
		NewManager:     config.NewManager,
		NewClient:      llm.NewClient,
		LaunchConfigUI: ui.LaunchConfigUI,
	}
}

func Execute() {
	rootCmd := NewRootCmd(defaultDependencies())
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func NewRootCmd(deps Dependencies) *cobra.Command {
	options := &rootOptions{}

	rootCmd := &cobra.Command{
		Use:           "aicommit",
		Short:         "AI-powered Git commit message generator",
		Long:          "AICommit analyzes your git diff and generates intelligent commit messages using Anthropic, Gemini, or OpenAI.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCommit(cmd, deps, options)
		},
	}

	rootCmd.PersistentFlags().StringVar(&options.ConfigPath, "config", "", "Path to the AICommit config file")
	rootCmd.Flags().BoolVar(&options.Push, "push", false, "Push to git remote after committing")
	rootCmd.Flags().BoolVar(&options.Clasp, "clasp", false, "Push to clasp after committing")
	rootCmd.Flags().BoolVar(&options.Wrangler, "wrangler", false, "Deploy to Cloudflare Workers after committing")
	rootCmd.Flags().BoolVar(&options.Export, "export", false, "Export diff to a file without committing")
	rootCmd.AddCommand(newConfigCmd(deps, options))

	return rootCmd
}

func runCommit(cmd *cobra.Command, deps Dependencies, options *rootOptions) error {
	if !git.IsGitRepository() {
		return fmt.Errorf("not in a git repository")
	}

	if options.Clasp {
		if !git.IsClaspProject() {
			return fmt.Errorf("not in a clasp repository (.clasp.json not found)")
		}

		confirmed, err := prompt.Confirm("Have you pulled from clasp?", false)
		if err != nil {
			return err
		}
		if !confirmed {
			color.Yellow("Please run 'clasp pull' first, then try again")
			return nil
		}
	}

	if options.Wrangler {
		if !git.IsWranglerProject() {
			return fmt.Errorf("not in a wrangler project (wrangler.toml not found)")
		}
	}

	manager, err := deps.NewManager(options.ConfigPath)
	if err != nil {
		return err
	}

	cfg, err := manager.Load()
	if err != nil {
		if errors.Is(err, config.ErrConfigNotFound) {
			return fmt.Errorf("configuration not found; run `aicommit config ui` to set up AICommit")
		}
		return err
	}

	apiKey, err := manager.ActiveAPIKey(cfg)
	if err != nil {
		return fmt.Errorf("%w; run `aicommit config ui` to add a key", err)
	}

	color.Yellow("Analyzing changes...")
	diff, err := git.GetFullDiff()
	if err != nil {
		return fmt.Errorf("failed to get diff: %w", err)
	}
	if diff == "" {
		color.Green("No changes to commit")
		return nil
	}

	if options.Export {
		exportFile := "git-diff-export.txt"
		if err := os.WriteFile(exportFile, []byte(diff), 0644); err != nil {
			return fmt.Errorf("failed to export diff: %w", err)
		}
		color.Green("Diff exported to: %s", exportFile)
		return nil
	}

	client, err := deps.NewClient(cfg.Provider, cfg.Model, apiKey)
	if err != nil {
		return fmt.Errorf("failed to initialize AI client: %w", err)
	}

	color.Yellow("Getting AI suggestion...")
	suggestion, err := llm.GenerateWithRetry(client, llm.TruncateDiff(diff, cfg.MaxDiffLength), 3)
	if err != nil {
		return fmt.Errorf("failed to generate commit message: %w", err)
	}

	currentMessage := suggestion
	for {
		fmt.Println()
		color.Cyan("--- SUGGESTED COMMIT MESSAGE ---")
		color.White("HEADER: %s", currentMessage.Header)
		if currentMessage.Description != "" {
			color.White("DESCRIPTION: %s", currentMessage.Description)
		}
		color.Cyan("--- END MESSAGE ---")
		fmt.Println()

		choice, err := prompt.Select("Use this message?", []string{"yes", "edit", "cancel"})
		if err != nil {
			return err
		}

		switch choice {
		case "cancel":
			color.Yellow("Commit cancelled")
			return nil
		case "edit":
			edited, err := prompt.EditCommitMessage(currentMessage)
			if err != nil {
				return err
			}
			currentMessage = edited
		case "yes":
			color.Yellow("Staging changes...")
			if err := git.StageAll(); err != nil {
				return fmt.Errorf("failed to stage changes: %w", err)
			}

			color.Yellow("Committing...")
			if err := git.Commit(currentMessage.Format()); err != nil {
				return fmt.Errorf("failed to commit: %w", err)
			}

			color.Green("\nCommit successful!")
			if lastCommit, err := git.GetLastCommit(); err == nil && lastCommit != "" {
				color.Cyan("Created: %s", lastCommit)
			}

			return runPostCommitOperations(options)
		}
	}
}

func runPostCommitOperations(options *rootOptions) error {
	var failures []string

	if options.Push {
		color.Yellow("Pushing to remote...")
		if err := git.Push(); err != nil {
			color.Red("Push failed: %v", err)
			failures = append(failures, "git push")
		} else {
			color.Green("Push successful!")
		}
	}

	if options.Clasp {
		color.Yellow("Pushing to clasp...")
		if err := git.ClaspPush(); err != nil {
			color.Red("Clasp push failed: %v", err)
			failures = append(failures, "clasp push")
		} else {
			color.Green("Clasp push successful!")
		}
	}

	if options.Wrangler {
		color.Yellow("Deploying to Cloudflare Workers...")
		if err := git.WranglerDeploy(); err != nil {
			color.Red("Wrangler deployment failed: %v", err)
			failures = append(failures, "wrangler deploy")
		} else {
			color.Green("Wrangler deployment successful!")
		}
	}

	if len(failures) > 0 {
		return fmt.Errorf("commit succeeded, but the following post-commit steps failed: %s", strings.Join(failures, ", "))
	}

	return nil
}

func newConfigCmd(deps Dependencies, options *rootOptions) *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage AICommit configuration and secrets",
	}

	configCmd.AddCommand(
		newConfigUICmd(deps, options),
		newConfigShowCmd(deps, options),
		newConfigSetCmd(deps, options),
		newConfigSetKeyCmd(deps, options),
		newConfigDeleteKeyCmd(deps, options),
	)

	return configCmd
}

func newConfigUICmd(deps Dependencies, options *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "ui",
		Short: "Open the configuration UI in your browser",
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := deps.NewManager(options.ConfigPath)
			if err != nil {
				return err
			}
			return deps.LaunchConfigUI(manager, cmd.OutOrStdout())
		},
	}
}

func newConfigShowCmd(deps Dependencies, options *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show current configuration and key presence",
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := deps.NewManager(options.ConfigPath)
			if err != nil {
				return err
			}
			summary, err := manager.Summary()
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Config path: %s\n", summary.ConfigPath)
			fmt.Fprintf(cmd.OutOrStdout(), "Config exists: %t\n", summary.ConfigExists)
			fmt.Fprintf(cmd.OutOrStdout(), "Provider: %s\n", summary.Provider)
			fmt.Fprintf(cmd.OutOrStdout(), "Model: %s\n", summary.Model)
			fmt.Fprintf(cmd.OutOrStdout(), "Max diff length: %d\n", summary.MaxDiffLength)
			for _, item := range provider.Providers() {
				fmt.Fprintf(cmd.OutOrStdout(), "Key stored for %s: %t\n", item, summary.KeyPresence[item])
			}
			return nil
		},
	}
}

func newConfigSetCmd(deps Dependencies, options *rootOptions) *cobra.Command {
	setCmd := &cobra.Command{
		Use:   "set",
		Short: "Update non-secret configuration values",
	}

	setCmd.AddCommand(
		&cobra.Command{
			Use:   "provider <anthropic|gemini|openai>",
			Short: "Set the active provider",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				manager, err := deps.NewManager(options.ConfigPath)
				if err != nil {
					return err
				}
				cfg, err := manager.LoadOrDefault()
				if err != nil {
					return err
				}
				providerValue, err := provider.ParseProvider(args[0])
				if err != nil {
					return err
				}
				cfg.Provider = providerValue
				cfg.Model = provider.DefaultModel(providerValue)
				if err := manager.Save(cfg); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Provider set to %s with default model %s\n", cfg.Provider, cfg.Model)
				return nil
			},
		},
		&cobra.Command{
			Use:   "model <model>",
			Short: "Set the active model",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				manager, err := deps.NewManager(options.ConfigPath)
				if err != nil {
					return err
				}
				cfg, err := manager.LoadOrDefault()
				if err != nil {
					return err
				}
				cfg.Model = args[0]
				if err := manager.Save(cfg); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Model set to %s\n", cfg.Model)
				return nil
			},
		},
		&cobra.Command{
			Use:   "max-diff-length <int>",
			Short: "Set the maximum diff length sent to the model",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				manager, err := deps.NewManager(options.ConfigPath)
				if err != nil {
					return err
				}
				cfg, err := manager.LoadOrDefault()
				if err != nil {
					return err
				}
				value, err := strconv.Atoi(args[0])
				if err != nil {
					return fmt.Errorf("max-diff-length must be an integer")
				}
				cfg.MaxDiffLength = value
				if err := manager.Save(cfg); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Max diff length set to %d\n", cfg.MaxDiffLength)
				return nil
			},
		},
	)

	return setCmd
}

func newConfigSetKeyCmd(deps Dependencies, options *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "set-key <provider> [key]",
		Short: "Store an API key securely for a provider",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := deps.NewManager(options.ConfigPath)
			if err != nil {
				return err
			}
			key := ""
			if len(args) == 2 {
				key = args[1]
			} else {
				key, err = prompt.Password(fmt.Sprintf("Enter API key for %s", args[0]))
				if err != nil {
					return err
				}
			}
			if err := manager.SetKey(args[0], key); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Stored API key for %s\n", args[0])
			return nil
		},
	}
}

func newConfigDeleteKeyCmd(deps Dependencies, options *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "delete-key <provider>",
		Short: "Delete a stored API key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := deps.NewManager(options.ConfigPath)
			if err != nil {
				return err
			}
			if err := manager.DeleteKey(args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted API key for %s\n", args[0])
			return nil
		},
	}
}
