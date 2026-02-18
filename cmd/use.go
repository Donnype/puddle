package cmd

import (
	"fmt"
	"os"

	"puddle/internal/lang"

	"github.com/spf13/cobra"
)

var flagClear bool

var useCmd = &cobra.Command{
	Use:   "use [language]",
	Short: "Set default language and versions for this shell session",
	Long: `Print export statements that set PUDDLE_* environment variables.
Wrap in eval to apply:

  eval "$(puddle use python -d 1.3.0 -r 3.11)"

Subsequent puddle commands will use these defaults.
Clear with:

  eval "$(puddle use --clear)"`,
	Args: cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if flagClear {
			fmt.Println("unset PUDDLE_LANG PUDDLE_DUCKDB_VERSION PUDDLE_RUNTIME_VERSION PUDDLE_LIB_VERSION;")
			fmt.Fprintln(os.Stderr, "Puddle environment cleared.")
			return nil
		}

		if len(args) == 0 {
			return fmt.Errorf("language required (or use --clear)")
		}

		l, err := lang.Get(args[0])
		if err != nil {
			return err
		}

		duckdbVer, rtVer, libVer := resolveVersions(l)

		fmt.Printf("export PUDDLE_LANG=%s;", args[0])
		fmt.Printf(" export PUDDLE_DUCKDB_VERSION=%s;", duckdbVer)
		fmt.Printf(" export PUDDLE_RUNTIME_VERSION=%s;", rtVer)
		if libVer != "" {
			fmt.Printf(" export PUDDLE_LIB_VERSION=%s;", libVer)
		}
		fmt.Println()

		fmt.Fprintf(os.Stderr, "Set: %s %s / DuckDB %s", l.Name, rtVer, duckdbVer)
		if libVer != "" {
			fmt.Fprintf(os.Stderr, " / lib %s", libVer)
		}
		fmt.Fprintln(os.Stderr)

		return nil
	},
}

func init() {
	useCmd.Flags().StringVarP(&flagDuckDBVersion, "duckdb-version", "d", "", "DuckDB version")
	useCmd.Flags().StringVarP(&flagRuntimeVersion, "runtime-version", "r", "", "language runtime version")
	useCmd.Flags().StringVarP(&flagLibVersion, "lib-version", "l", "", "language library version")
	useCmd.Flags().BoolVar(&flagClear, "clear", false, "clear puddle environment variables")
}
