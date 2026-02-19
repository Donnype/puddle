package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "puddle",
	Short: "Spin up DuckDB with any language binding via Docker",
	Long: `puddle builds and runs Docker containers for DuckDB language bindings.

It ships self-contained Dockerfiles for Go, Python, Java, Node.js,
Rust, PHP, and Ruby — each with a built-in SQL REPL.

Supports DuckDB versions from 1.2.x to 1.4.4 and both amd64/arm64.`,
	SilenceUsage: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(cleanCmd)
	rootCmd.AddCommand(shellCmd)
	rootCmd.AddCommand(useCmd)
	rootCmd.AddCommand(showCmd)
}
