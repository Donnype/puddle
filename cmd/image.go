package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"puddle/internal/config"
	"puddle/internal/docker"
	"puddle/internal/lang"

	"github.com/spf13/cobra"
)

var (
	flagDuckDBVersion  string
	flagRuntimeVersion string
)

func addVersionFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&flagDuckDBVersion, "duckdb-version", "d", "", "DuckDB version")
	cmd.Flags().StringVarP(&flagRuntimeVersion, "runtime-version", "r", "", "language runtime version (e.g. Python 3.11, Java 17)")
}

// resolveLang returns the language name from args or global config.
func resolveLang(args []string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}
	if global := config.LoadGlobal(); global.Lang != "" {
		return global.Lang, nil
	}
	return "", fmt.Errorf("no language specified (set a default with: puddle use <language> --global)")
}

// resolveVersions returns the DuckDB and runtime versions using:
// CLI flags > global config > language registry defaults.
func resolveVersions(l lang.Language) (string, string) {
	global := config.LoadGlobal()
	return firstNonEmpty(flagDuckDBVersion, global.DuckDBVersion, l.DefaultDuckDB),
		firstNonEmpty(flagRuntimeVersion, global.RuntimeVersion, l.DefaultRuntime)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// ensureImage pulls a pre-built image from GHCR if not already cached.
func ensureImage(ctx context.Context, langName string) (string, error) {
	l, err := lang.Get(langName)
	if err != nil {
		return "", err
	}

	duckdbVer, rtVer := resolveVersions(l)
	tag := imageTag(langName, duckdbVer, rtVer)

	if err := docker.Ping(ctx); err != nil {
		return "", err
	}

	if docker.ImageExists(ctx, tag) {
		return tag, nil
	}

	remoteRef := fmt.Sprintf("%s/%s", docker.GHCRPrefix, tag)
	fmt.Fprintf(os.Stderr, "Pulling %s...\n", remoteRef)
	err = retryOnRateLimit(ctx, func() error {
		return docker.Pull(ctx, docker.PullOptions{
			RemoteRef: remoteRef,
			LocalTag:  tag,
		})
	})
	if err != nil {
		return "", fmt.Errorf("pulling image: %w", err)
	}
	return tag, nil
}

// retryOnRateLimit retries fn up to 3 times with backoff when a Docker rate
// limit error is detected (HTTP 429 / "toomanyrequests").
func retryOnRateLimit(ctx context.Context, fn func() error) error {
	backoff := [3]time.Duration{10 * time.Second, 30 * time.Second, 60 * time.Second}
	var err error
	for attempt := range 4 {
		err = fn()
		if err == nil || !isRateLimitError(err) || attempt == 3 {
			return err
		}
		wait := backoff[attempt]
		fmt.Fprintf(os.Stderr, "Rate limited, retrying in %s...\n", wait)
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return err
}

func isRateLimitError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "toomanyrequests") || strings.Contains(msg, "rate limit")
}

func imageTag(langName, duckdbVer, runtimeVer string) string {
	return fmt.Sprintf("puddle-%s:%s-%s",
		langName,
		firstNonEmpty(duckdbVer, "latest"),
		firstNonEmpty(runtimeVer, "default"),
	)
}
