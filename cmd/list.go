package cmd

import (
	"fmt"

	"puddle/internal/lang"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List available language bindings",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Available languages:")
		fmt.Println()

		for _, name := range lang.Names() {
			l := lang.Registry[name]
			fmt.Printf("  %-10s %-10s runtime=%s, duckdb=%s\n", name, l.Name, l.DefaultRuntime, l.DefaultDuckDB)
			if l.VersionRange != "" {
				fmt.Printf("             %s\n", l.VersionRange)
			}
		}
		return nil
	},
}
