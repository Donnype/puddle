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
	Short: "Remove all puddle Docker images",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
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
	},
}

func init() {
	cleanCmd.Flags().BoolVarP(&flagForce, "force", "f", false, "force removal of images")
}
