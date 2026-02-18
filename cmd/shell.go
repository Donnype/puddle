package cmd

import (
	"github.com/spf13/cobra"
)

var shellCmd = &cobra.Command{
	Use:   "shell [language]",
	Short: "Start a bash shell in a language container with PWD mounted",
	Long: `Start a bash shell inside a container for the specified language binding.
The current directory is mounted at /work inside the container.

The language can be omitted if PUDDLE_LANG is set (see "puddle use").

This is equivalent to "puddle run <language>" (without --repl).`,
	Args: cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		langName, err := resolveLang(args)
		if err != nil {
			return err
		}
		return runShellMode(cmd.Context(), langName)
	},
}

func init() {
	addBuildFlags(shellCmd)
	shellCmd.Flags().StringArrayVarP(&flagEnv, "env", "e", nil, "set environment variables (KEY=VALUE), can be repeated")
}
