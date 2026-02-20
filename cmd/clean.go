package cmd

import (
	"fmt"
	"os"
	"strings"

	"puddle/internal/config"
	"puddle/internal/docker"

	"github.com/spf13/cobra"
)

var (
	flagForce    bool
	flagSessions bool
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove puddle Docker images and stale sessions",
	Long: `Remove all puddle Docker images.

Use --sessions to also clean up stale unnamed session files
from ~/.config/puddle/sessions/.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if flagSessions {
			return cleanSessions()
		}
		return cleanImages(cmd)
	},
}

func init() {
	cleanCmd.Flags().BoolVarP(&flagForce, "force", "f", false, "force removal of images")
	cleanCmd.Flags().BoolVar(&flagSessions, "sessions", false, "clean stale unnamed session files instead of images")
}

func cleanImages(cmd *cobra.Command) error {
	cli, err := docker.New()
	if err != nil {
		return err
	}
	defer cli.Close()

	ctx := cmd.Context()

	if err := cli.Ping(ctx); err != nil {
		return err
	}

	images, err := cli.ListImages(ctx)
	if err != nil {
		return err
	}

	if len(images) == 0 {
		fmt.Fprintln(os.Stderr, "No puddle images found.")
		return nil
	}

	fmt.Fprintln(os.Stderr, "Removing puddle images:")
	for _, img := range images {
		fmt.Fprintf(os.Stderr, "  %s\n", strings.Join(img.Tags, ", "))
	}

	var failed int
	for _, img := range images {
		if err := cli.RemoveImage(ctx, img.ID, flagForce); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: %v\n", err)
			failed++
		}
	}

	removed := len(images) - failed
	fmt.Fprintf(os.Stderr, "Removed %d image(s).\n", removed)
	if failed > 0 {
		return fmt.Errorf("failed to remove %d image(s) (try --force)", failed)
	}
	return nil
}

func cleanSessions() error {
	sessions, err := config.ListSessions()
	if err != nil {
		return fmt.Errorf("listing sessions: %w", err)
	}
	if len(sessions) == 0 {
		fmt.Fprintln(os.Stderr, "No session files found.")
		return nil
	}

	var named, removed int
	for _, id := range sessions {
		if config.IsNamedSession(id) {
			named++
			continue
		}
		if config.RemoveSession(id) == nil {
			removed++
		}
	}

	if removed == 0 {
		fmt.Fprintf(os.Stderr, "No stale unnamed sessions found (%d named session(s) preserved).\n", named)
	} else {
		fmt.Fprintf(os.Stderr, "Removed %d stale session(s).\n", removed)
		if named > 0 {
			fmt.Fprintf(os.Stderr, "%d named session(s) preserved.\n", named)
		}
	}
	return nil
}
