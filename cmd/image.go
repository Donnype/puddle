package cmd

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"

	"puddle/dockerfiles"
	"puddle/internal/config"
	"puddle/internal/docker"
	"puddle/internal/lang"

	"github.com/spf13/cobra"
)

var (
	flagDuckDBVersion  string
	flagArch           string
	flagLibVersion     string
	flagRuntimeVersion string
	flagBuild          bool
)

func addVersionFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&flagDuckDBVersion, "duckdb-version", "d", "", "DuckDB version")
	cmd.Flags().StringVarP(&flagRuntimeVersion, "runtime-version", "r", "", "language runtime version (e.g. Python 3.11, Java 17)")
	cmd.Flags().StringVarP(&flagLibVersion, "lib-version", "l", "", "language library version (e.g. PHP duckdb lib)")
}

func addBuildFlags(cmd *cobra.Command) {
	addVersionFlags(cmd)
	cmd.Flags().StringVarP(&flagArch, "arch", "a", "", "target architecture: amd64, arm64")
	cmd.Flags().BoolVar(&flagBuild, "build", false, "force a local build, skip pulling from GHCR")
}

// resolveLang returns the language name from args, session, or global config.
func resolveLang(args []string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}
	if sess := config.LoadSession(); sess.Lang != "" {
		return sess.Lang, nil
	}
	if global := config.LoadGlobal(); global.Lang != "" {
		return global.Lang, nil
	}
	return "", fmt.Errorf("no language specified (start a session with: puddle use <language>)")
}

// resolveVersions returns the DuckDB, runtime, and lib versions using:
// CLI flags > session config > global config > language registry defaults.
func resolveVersions(l lang.Language) (string, string, string) {
	return resolveVersionsFrom(l, true)
}

// resolveVersionsNoSession resolves versions skipping the current session.
// Used by "puddle use" when creating a new session.
func resolveVersionsNoSession(l lang.Language) (string, string, string) {
	return resolveVersionsFrom(l, false)
}

func resolveVersionsFrom(l lang.Language, includeSession bool) (string, string, string) {
	var sess config.Config
	if includeSession {
		sess = config.LoadSession()
	}
	global := config.LoadGlobal()

	return firstNonEmpty(flagDuckDBVersion, sess.DuckDBVersion, global.DuckDBVersion, l.DefaultDuckDB),
		firstNonEmpty(flagRuntimeVersion, sess.RuntimeVersion, global.RuntimeVersion, l.DefaultRuntime),
		firstNonEmpty(flagLibVersion, sess.LibVersion, global.LibVersion, l.DefaultLib)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// ensureImage pulls a pre-built image from GHCR, or builds locally with --build.
func ensureImage(ctx context.Context, langName string) (string, error) {
	l, err := lang.Get(langName)
	if err != nil {
		return "", err
	}

	duckdbVer, rtVer, _ := resolveVersions(l)

	if flagBuild || flagArch != "" || flagLibVersion != "" {
		return buildImage(ctx, langName)
	}

	tag := imageTag(langName, duckdbVer, rtVer)

	cli, err := docker.New()
	if err != nil {
		return "", err
	}
	defer cli.Close()

	if err := cli.Ping(ctx); err != nil {
		return "", err
	}

	if cli.ImageExists(ctx, tag) {
		return tag, nil
	}

	remoteRef := fmt.Sprintf("%s/%s", docker.GHCRPrefix, tag)
	fmt.Fprintf(os.Stderr, "Pulling %s...\n", remoteRef)
	err = retryOnRateLimit(ctx, func() error {
		return cli.Pull(ctx, docker.PullOptions{
			RemoteRef: remoteRef,
			LocalTag:  tag,
		})
	})
	if err != nil {
		return "", fmt.Errorf("pulling image: %w\n(use --build to build locally)", err)
	}
	return tag, nil
}

// buildImage builds the Docker image and returns the image tag.
func buildImage(ctx context.Context, langName string) (string, error) {
	l, err := lang.Get(langName)
	if err != nil {
		return "", err
	}

	cli, err := docker.New()
	if err != nil {
		return "", err
	}
	defer cli.Close()

	if err := cli.Ping(ctx); err != nil {
		return "", err
	}

	contextFS, err := fs.Sub(dockerfiles.FS, l.Dir)
	if err != nil {
		return "", fmt.Errorf("loading dockerfiles for %s: %w", langName, err)
	}

	duckdbVer, rtVer, libVer := resolveVersions(l)

	buildArgs := make(map[string]string)
	if duckdbVer != "" && l.DuckDBVersionArg != "" {
		buildArgs[l.DuckDBVersionArg] = duckdbVer
	}
	if libVer != "" && l.LibVersionArg != "" {
		buildArgs[l.LibVersionArg] = libVer
	}
	if rtVer != "" && l.RuntimeVersionArg != "" {
		buildArgs[l.RuntimeVersionArg] = rtVer
	}

	var platform string
	if flagArch != "" {
		platform = "linux/" + flagArch
	}

	tag := imageTag(langName, duckdbVer, rtVer)

	fmt.Fprintf(os.Stderr, "Building %s %s with DuckDB %s...\n", l.Name, rtVer, duckdbVer)
	err = retryOnRateLimit(ctx, func() error {
		return cli.Build(ctx, docker.BuildOptions{
			ContextFS: contextFS,
			Tag:       tag,
			Platform:  platform,
			BuildArgs: buildArgs,
		})
	})
	if err != nil {
		return "", err
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
	tag := fmt.Sprintf("puddle-%s:%s-%s",
		langName,
		firstNonEmpty(duckdbVer, "latest"),
		firstNonEmpty(runtimeVer, "default"),
	)
	if flagArch != "" {
		tag += "-" + flagArch
	}
	return tag
}
