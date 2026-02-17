package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"puddle/internal/docker"

	"github.com/spf13/cobra"
)

var (
	flagREPL   bool
	flagNative bool
	flagBinary string
	flagEnv    []string
	flagSQLCmd string
)

var runCmd = &cobra.Command{
	Use:   "run <language> [sql_file]",
	Short: "Build and run a DuckDB SQL REPL for a language binding",
	Long: `Build a Docker image for the specified language binding and start
an interactive DuckDB SQL REPL inside it.

The REPL speaks SQL through the language's native DuckDB binding.
Use .quit or .exit to leave the REPL.

Use -c to execute a SQL command and exit:
  puddle run python -c "SELECT 42;"

Use -c - to read SQL from stdin:
  echo "SELECT 42;" | puddle run python -c -

Pass a SQL file as a second argument:
  puddle run python query.sql

Use --native to run without Docker, using the host's language runtime.
Use --binary to override the default runtime binary in native mode.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		langName := args[0]

		// Determine SQL input source.
		var sqlReader io.Reader
		if flagSQLCmd != "" && len(args) == 2 {
			return fmt.Errorf("cannot use both -c and a SQL file argument")
		}
		if flagSQLCmd != "" {
			if flagSQLCmd == "-" {
				sqlReader = os.Stdin
			} else {
				sqlReader = strings.NewReader(flagSQLCmd + "\n")
			}
		} else if len(args) == 2 {
			data, err := os.ReadFile(args[1])
			if err != nil {
				return fmt.Errorf("reading SQL file: %w", err)
			}
			sqlReader = strings.NewReader(string(data))
		}

		if flagNative {
			return runNative(cmd.Context(), langName, flagBinary, sqlReader)
		}

		tag, err := buildImage(cmd.Context(), langName)
		if err != nil {
			return err
		}

		cli, err := docker.New()
		if err != nil {
			return err
		}
		defer cli.Close()

		// Forward MOTHERDUCK_TOKEN if set, plus any user-supplied env vars.
		var env []string
		if token := os.Getenv("MOTHERDUCK_TOKEN"); token != "" {
			env = append(env, "MOTHERDUCK_TOKEN="+token)
		}
		env = append(env, flagEnv...)

		if sqlReader == nil {
			fmt.Fprintf(os.Stderr, "\nStarting REPL...\n")
		}
		return cli.Run(cmd.Context(), docker.RunOptions{
			Image: tag,
			Env:   env,
			Stdin: sqlReader,
		})
	},
}

func init() {
	addBuildFlags(runCmd)
	runCmd.Flags().BoolVar(&flagREPL, "repl", true, "start the SQL REPL (default)")
	runCmd.Flags().MarkHidden("repl")
	runCmd.Flags().BoolVarP(&flagNative, "native", "n", false, "run without Docker using the host's runtime")
	runCmd.Flags().StringVarP(&flagBinary, "binary", "b", "", "path to the language runtime binary (native mode)")
	runCmd.Flags().StringArrayVarP(&flagEnv, "env", "e", nil, "set environment variables (KEY=VALUE), can be repeated")
	runCmd.Flags().StringVarP(&flagSQLCmd, "command", "c", "", "execute a SQL command and exit (use - to read from stdin)")
}
