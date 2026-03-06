package cmd

import (
	"fmt"
	"os"
	"strings"

	"puddle/internal/docker"

	"github.com/spf13/cobra"
)

var flagForce bool

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove puddle Docker images",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		if err := docker.Ping(ctx); err != nil {
			return err
		}

		images, err := docker.ListImages(ctx)
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
			// Remove each tag individually rather than by ID.
			// Images pulled from GHCR have both a remote and local tag;
			// "docker rmi <id>" refuses to remove multi-tagged images without --force.
			for _, tag := range img.Tags {
				if err := docker.RemoveImage(ctx, tag, flagForce); err != nil {
					fmt.Fprintf(os.Stderr, "  warning: %v\n", err)
					failed++
				}
			}
		}

		fmt.Fprintf(os.Stderr, "Removed %d image(s).\n", len(images))
		if failed > 0 {
			return fmt.Errorf("failed to remove %d tag(s) (try --force)", failed)
		}
		return nil
	},
}

func init() {
	cleanCmd.Flags().BoolVarP(&flagForce, "force", "f", false, "force removal of images")
}
