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
  session  — from the active session file (puddle use)
  global   — from ~/.config/puddle/config.json (puddle use --global)
  default  — built-in registry default`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		sess := config.LoadSession()
		global := config.LoadGlobal()
		sessionID := config.ActiveSessionID()

		// Resolve language.
		langName, langSource := "", ""
		if sess.Lang != "" {
			langName, langSource = sess.Lang, "session"
		} else if global.Lang != "" {
			langName, langSource = global.Lang, "global"
		}

		if langName == "" {
			fmt.Println("No puddle configuration active.")
			fmt.Println()
			fmt.Println("Start a session:   puddle use <language>")
			fmt.Println("Set global default: puddle use <language> --global")
			return nil
		}

		l, err := lang.Get(langName)
		if err != nil {
			fmt.Printf("Language: %s (%s) — unknown language\n", langName, langSource)
			return nil
		}

		// Resolve each version with source tracking.
		duckdbVer, duckdbSrc := resolveWithSource(
			sess.DuckDBVersion, global.DuckDBVersion, l.DefaultDuckDB,
		)
		rtVer, rtSrc := resolveWithSource(
			sess.RuntimeVersion, global.RuntimeVersion, l.DefaultRuntime,
		)
		libVer, libSrc := resolveWithSource(
			sess.LibVersion, global.LibVersion, l.DefaultLib,
		)

		// Session info.
		if sessionID != "" {
			named := config.IsNamedSession(sessionID)
			if named {
				fmt.Printf("Session:         %s\n", sessionID)
			} else {
				fmt.Printf("Session:         %s (unnamed)\n", sessionID)
			}
			fmt.Printf("                 %s\n", config.SessionPath(sessionID))
		}

		fmt.Printf("Language:        %-12s  ← %s\n", langName+" ("+l.Name+")", langSource)
		fmt.Printf("DuckDB version:  %-12s  ← %s\n", duckdbVer, duckdbSrc)
		fmt.Printf("Runtime version: %-12s  ← %s\n", rtVer, rtSrc)
		if libVer != "" {
			fmt.Printf("Lib version:     %-12s  ← %s\n", libVer, libSrc)
		}

		return nil
	},
}

// resolveWithSource returns the resolved value and its source label.
func resolveWithSource(sessVal, globalVal, defaultVal string) (string, string) {
	if sessVal != "" {
		return sessVal, "session"
	}
	if globalVal != "" {
		return globalVal, "global"
	}
	if defaultVal != "" {
		return defaultVal, "default"
	}
	return "", ""
}
