package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"puddle/internal/config"
	"puddle/internal/lang"

	"github.com/spf13/cobra"
)

var (
	flagSessionName string
	flagGlobal      bool
	flagClear       bool
)

var useCmd = &cobra.Command{
	Use:   "use [language]",
	Short: "Start a puddle session with a language and version config",
	Long: `Start a new shell session with the specified language and versions.

  puddle use python -d 1.3.0          # unnamed session (cleaned up on exit)
  puddle use python --name prod       # named session (persists after exit)
  puddle use python --global          # set global default (no subshell)
  puddle use --clear                  # clear global config

The subshell inherits your environment. Type 'exit' to end the session.
Inside the session, puddle commands automatically use the session config.`,
	Args: cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if flagClear {
			if err := config.ClearGlobal(); err != nil {
				return fmt.Errorf("clearing global config: %w", err)
			}
			fmt.Fprintln(os.Stderr, "Global puddle config cleared.")
			return nil
		}

		if len(args) == 0 {
			return fmt.Errorf("language required (or use --clear)")
		}

		l, err := lang.Get(args[0])
		if err != nil {
			return err
		}

		duckdbVer, rtVer, libVer := resolveVersionsNoSession(l)

		cfg := config.Config{
			Lang:           args[0],
			DuckDBVersion:  duckdbVer,
			RuntimeVersion: rtVer,
			LibVersion:     libVer,
		}

		// --global: write config file, no subshell.
		if flagGlobal {
			if err := config.SaveGlobal(cfg); err != nil {
				return fmt.Errorf("saving global config: %w", err)
			}
			fmt.Fprintf(os.Stderr, "Global default: %s %s / DuckDB %s", l.Name, rtVer, duckdbVer)
			if libVer != "" {
				fmt.Fprintf(os.Stderr, " / lib %s", libVer)
			}
			fmt.Fprintf(os.Stderr, "\nWritten to %s\n", config.GlobalPath())
			return nil
		}

		// Check for nested sessions.
		if existing := config.ActiveSessionID(); existing != "" {
			fmt.Fprintf(os.Stderr, "Warning: already inside session %q, starting a nested session.\n", existing)
		}

		// Create session file.
		id, err := config.SaveSession(flagSessionName, cfg)
		if err != nil {
			return fmt.Errorf("creating session: %w", err)
		}

		named := flagSessionName != ""

		// Print session info.
		label := id
		if !named {
			label = id + " (unnamed)"
		}
		fmt.Fprintf(os.Stderr, "Session %s: %s %s / DuckDB %s", label, l.Name, rtVer, duckdbVer)
		if libVer != "" {
			fmt.Fprintf(os.Stderr, " / lib %s", libVer)
		}
		fmt.Fprintln(os.Stderr)

		// Launch subshell.
		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/sh"
		}

		shellCmd := exec.Command(shell)
		shellCmd.Env = append(os.Environ(), "PUDDLE_SESSION="+id)
		shellCmd.Stdin = os.Stdin
		shellCmd.Stdout = os.Stdout
		shellCmd.Stderr = os.Stderr

		// Forward signals to the child shell.
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			for sig := range sigCh {
				if shellCmd.Process != nil {
					shellCmd.Process.Signal(sig)
				}
			}
		}()

		fmt.Fprintln(os.Stderr, "Type 'exit' to end the session.")
		err = shellCmd.Run()
		signal.Stop(sigCh)
		close(sigCh)

		// Clean up unnamed sessions before any exit path.
		if !named {
			config.RemoveSession(id)
			fmt.Fprintln(os.Stderr, "Session ended, config cleaned up.")
		} else {
			fmt.Fprintf(os.Stderr, "Session %s ended (config preserved).\n", id)
		}

		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				os.Exit(exitErr.ExitCode())
			}
			return err
		}
		return nil
	},
}

func init() {
	addVersionFlags(useCmd)
	useCmd.Flags().StringVar(&flagSessionName, "name", "", "name the session (persists after exit)")
	useCmd.Flags().BoolVar(&flagGlobal, "global", false, "set global default instead of starting a session")
	useCmd.Flags().BoolVar(&flagClear, "clear", false, "clear global config")
}
