package cmd

import (
	"fmt"
	"os"

	"puddle/internal/lang"

	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current puddle configuration",
	Long:  `Display the active PUDDLE_* environment variables set by "puddle use".`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		langName := os.Getenv("PUDDLE_LANG")
		if langName == "" {
			fmt.Println("No puddle environment set.")
			fmt.Println("Use: eval \"$(puddle use <language>)\"")
			return nil
		}

		l, err := lang.Get(langName)
		if err != nil {
			fmt.Printf("PUDDLE_LANG=%s (unknown language)\n", langName)
			return nil
		}

		duckdbVer := os.Getenv("PUDDLE_DUCKDB_VERSION")
		if duckdbVer == "" {
			duckdbVer = l.DefaultDuckDB
		}
		rtVer := os.Getenv("PUDDLE_RUNTIME_VERSION")
		if rtVer == "" {
			rtVer = l.DefaultRuntime
		}
		libVer := os.Getenv("PUDDLE_LIB_VERSION")
		if libVer == "" {
			libVer = l.DefaultLib
		}

		fmt.Printf("Language:        %s (%s)\n", langName, l.Name)
		fmt.Printf("DuckDB version:  %s\n", duckdbVer)
		fmt.Printf("Runtime version: %s\n", rtVer)
		if libVer != "" {
			fmt.Printf("Lib version:     %s\n", libVer)
		}

		return nil
	},
}
