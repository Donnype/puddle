package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"puddle/internal/docker"

	"github.com/spf13/cobra"
)

var (
	flagREPL   bool
	flagEnv    []string
	flagSQLCmd string
)

var runCmd = &cobra.Command{
	Use:   "run [language] [sql_file]",
	Short: "Start a shell (default) or DuckDB REPL for a language binding",
	Long: `Start a container for the specified language binding.

By default, opens a bash shell with the current directory mounted at /work.
Use --repl to start the interactive DuckDB SQL REPL instead.

The language can be omitted if a global default is set (see "puddle use --global").

Use -c to execute a SQL command and exit:
  puddle run python -c "SELECT 42;"

Use -c - to read SQL from stdin:
  echo "SELECT 42;" | puddle run python -c -

Pass a SQL file as a second argument:
  puddle run python query.sql`,
	Args: cobra.RangeArgs(0, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		langName, err := resolveLang(args)
		if err != nil {
			return err
		}
		// Shift args past the language name for SQL file detection.
		sqlArgs := args
		if len(args) > 0 {
			sqlArgs = args[1:]
		}

		// Determine SQL input source.
		var sqlReader io.Reader
		if flagSQLCmd != "" && len(sqlArgs) > 0 {
			return fmt.Errorf("cannot use both -c and a SQL file argument")
		}
		// Append ";\n.quit\n" so the REPL exits cleanly without relying
		// on stdin EOF (Docker attach CloseWrite is unreliable on macOS).
		// The extra ";" is harmless if the SQL already ends with one.
		if flagSQLCmd != "" {
			if flagSQLCmd == "-" {
				sqlReader = io.MultiReader(os.Stdin, strings.NewReader(";\n.quit\n"))
			} else {
				sqlReader = strings.NewReader(flagSQLCmd + ";\n.quit\n")
			}
		} else if len(sqlArgs) > 0 {
			data, err := os.ReadFile(sqlArgs[0])
			if err != nil {
				return fmt.Errorf("reading SQL file: %w", err)
			}
			sqlReader = strings.NewReader(string(data) + ";\n.quit\n")
		}

		// SQL mode (batch or interactive REPL).
		if sqlReader != nil || flagREPL {
			return runREPL(cmd.Context(), langName, sqlReader)
		}

		// Default: shell mode.
		return runShellMode(cmd.Context(), langName)
	},
}

func init() {
	addVersionFlags(runCmd)
	runCmd.Flags().BoolVar(&flagREPL, "repl", false, "start the DuckDB SQL REPL instead of a shell")
	runCmd.Flags().StringArrayVarP(&flagEnv, "env", "e", nil, "set environment variables (KEY=VALUE), can be repeated")
	runCmd.Flags().StringVarP(&flagSQLCmd, "command", "c", "", "execute a SQL command and exit (use - to read from stdin)")
}

// defaultEnv returns MOTHERDUCK_TOKEN (if set) plus any user-supplied -e vars.
func defaultEnv() []string {
	var env []string
	if token := os.Getenv("MOTHERDUCK_TOKEN"); token != "" {
		env = append(env, "MOTHERDUCK_TOKEN="+token)
	}
	env = append(env, flagEnv...)
	return env
}

// runREPL starts the DuckDB SQL REPL (or runs a SQL command/file).
func runREPL(ctx context.Context, langName string, sqlReader io.Reader) error {
	tag, err := ensureImage(ctx, langName)
	if err != nil {
		return err
	}

	if sqlReader == nil {
		fmt.Fprintf(os.Stderr, "\nStarting REPL...\n")
	}
	return docker.Run(ctx, docker.RunOptions{
		Image: tag,
		Env:   defaultEnv(),
		Stdin: sqlReader,
	})
}

// runShellMode starts a bash shell with PWD mounted at /work.
func runShellMode(ctx context.Context, langName string) error {
	tag, err := ensureImage(ctx, langName)
	if err != nil {
		return err
	}

	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	fmt.Fprintf(os.Stderr, "\nStarting shell (%s mounted at /work)...\n", pwd)
	return docker.Run(ctx, docker.RunOptions{
		Image:      tag,
		Env:        defaultEnv(),
		Cmd:        []string{"/bin/bash"},
		Binds:      []string{pwd + ":/work"},
		WorkingDir: "/work",
	})
}
