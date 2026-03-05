package cmd

import (
	"fmt"

	"puddle/internal/config"
	"puddle/internal/lang"

	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current puddle configuration",
	Long: `Display the resolved puddle configuration and where each value comes from.

Sources (highest to lowest priority):
  global   — from ~/.config/puddle/config.json (puddle use)
  default  — built-in registry default`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		global := config.LoadGlobal()

		if global.Lang == "" {
			fmt.Println("No puddle configuration active.")
			fmt.Println()
			fmt.Println("Set a default: puddle use <language>")
			return nil
		}

		l, err := lang.Get(global.Lang)
		if err != nil {
			fmt.Printf("Language: %s (global) — unknown language\n", global.Lang)
			return nil
		}

		duckdbVer, duckdbSrc := resolveWithSource(global.DuckDBVersion, l.DefaultDuckDB)
		rtVer, rtSrc := resolveWithSource(global.RuntimeVersion, l.DefaultRuntime)

		fmt.Printf("Language:        %-12s  ← global\n", global.Lang+" ("+l.Name+")")
		fmt.Printf("DuckDB version:  %-12s  ← %s\n", duckdbVer, duckdbSrc)
		fmt.Printf("Runtime version: %-12s  ← %s\n", rtVer, rtSrc)

		return nil
	},
}

// resolveWithSource returns the resolved value and its source label.
func resolveWithSource(globalVal, defaultVal string) (string, string) {
	if globalVal != "" {
		return globalVal, "global"
	}
	if defaultVal != "" {
		return defaultVal, "default"
	}
	return "", ""
}
