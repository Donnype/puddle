package cmd

import (
	"fmt"
	"os"

	"puddle/internal/docker"

	"github.com/spf13/cobra"
)

var (
	flagREPL   bool
	flagNative bool
	flagBinary string
	flagEnv    []string
)

var runCmd = &cobra.Command{
	Use:   "run <language>",
	Short: "Build and run a DuckDB SQL REPL for a language binding",
	Long: `Build a Docker image for the specified language binding and start
an interactive DuckDB SQL REPL inside it.

The REPL speaks SQL through the language's native DuckDB binding.
Use .quit or .exit to leave the REPL.

Use --native to run without Docker, using the host's language runtime.
Use --binary to override the default runtime binary in native mode.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		langName := args[0]

		if flagNative {
			return runNative(cmd.Context(), langName, flagBinary)
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

		fmt.Fprintf(os.Stderr, "\nStarting REPL...\n")
		return cli.Run(cmd.Context(), docker.RunOptions{
			Image: tag,
			Env:   env,
		})
	},
}

func init() {
	addBuildFlags(runCmd)
	runCmd.Flags().BoolVar(&flagREPL, "repl", true, "start the SQL REPL (default)")
	runCmd.Flags().MarkHidden("repl")
	runCmd.Flags().BoolVar(&flagNative, "native", false, "run without Docker using the host's runtime")
	runCmd.Flags().StringVar(&flagBinary, "binary", "", "path to the language runtime binary (native mode)")
	runCmd.Flags().StringArrayVarP(&flagEnv, "env", "e", nil, "set environment variables (KEY=VALUE), can be repeated")
}
