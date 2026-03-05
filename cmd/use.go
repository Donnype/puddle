package cmd

import (
	"fmt"
	"os"

	"puddle/internal/config"
	"puddle/internal/lang"

	"github.com/spf13/cobra"
)

var flagClear bool

var useCmd = &cobra.Command{
	Use:   "use [language]",
	Short: "Set or clear the global default language and versions",
	Long: `Set the global default language and version config.

  puddle use python                   # set default language
  puddle use python -d 1.3.0 -r 3.11 # set language + versions
  puddle use --clear                  # clear global config`,
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

		duckdbVer, rtVer := resolveVersions(l)

		cfg := config.Config{
			Lang:           args[0],
			DuckDBVersion:  duckdbVer,
			RuntimeVersion: rtVer,
		}

		if err := config.SaveGlobal(cfg); err != nil {
			return fmt.Errorf("saving global config: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Global default: %s %s / DuckDB %s\n", l.Name, rtVer, duckdbVer)
		fmt.Fprintf(os.Stderr, "Written to %s\n", config.GlobalPath())
		return nil
	},
}

func init() {
	addVersionFlags(useCmd)
	useCmd.Flags().BoolVar(&flagClear, "clear", false, "clear global config")
}
