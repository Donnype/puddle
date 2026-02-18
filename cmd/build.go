package cmd

import (
	"context"
	"fmt"
	"io/fs"
	"os"

	"puddle/dockerfiles"
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

var buildCmd = &cobra.Command{
	Use:   "build [language]",
	Short: "Build a Docker image for a language binding",
	Args:  cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		langName, err := resolveLang(args)
		if err != nil {
			return err
		}
		tag, err := ensureImage(cmd.Context(), langName)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "\nImage ready: %s\n", tag)
		return nil
	},
}

func init() {
	addBuildFlags(buildCmd)
}

func addBuildFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&flagDuckDBVersion, "duckdb-version", "d", "", "DuckDB version (uses language default if unset)")
	cmd.Flags().StringVarP(&flagArch, "arch", "a", "", "target architecture: amd64, arm64")
	cmd.Flags().StringVarP(&flagLibVersion, "lib-version", "l", "", "language library version (e.g. PHP duckdb lib)")
	cmd.Flags().StringVarP(&flagRuntimeVersion, "runtime-version", "r", "", "language runtime version (e.g. Python 3.11, Java 17)")
	cmd.Flags().BoolVar(&flagBuild, "build", false, "force a local build, skip pulling from GHCR")
}

// resolveLang returns the language name from args or PUDDLE_LANG env var.
func resolveLang(args []string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}
	if envLang := os.Getenv("PUDDLE_LANG"); envLang != "" {
		return envLang, nil
	}
	return "", fmt.Errorf("no language specified (set one with: eval \"$(puddle use <language>)\")")
}

// resolveVersions returns the DuckDB, runtime, and lib versions using:
// CLI flags > PUDDLE_* env vars > language registry defaults.
func resolveVersions(l lang.Language) (duckdbVer, rtVer, libVer string) {
	duckdbVer = flagDuckDBVersion
	if duckdbVer == "" {
		duckdbVer = os.Getenv("PUDDLE_DUCKDB_VERSION")
	}
	if duckdbVer == "" {
		duckdbVer = l.DefaultDuckDB
	}

	rtVer = flagRuntimeVersion
	if rtVer == "" {
		rtVer = os.Getenv("PUDDLE_RUNTIME_VERSION")
	}
	if rtVer == "" {
		rtVer = l.DefaultRuntime
	}

	libVer = flagLibVersion
	if libVer == "" {
		libVer = os.Getenv("PUDDLE_LIB_VERSION")
	}
	if libVer == "" {
		libVer = l.DefaultLib
	}

	return
}

// ensureImage pulls a pre-built image from GHCR, falling back to a local build.
// If --build is set or -l/-a are specified, it builds locally directly.
func ensureImage(ctx context.Context, langName string) (string, error) {
	l, err := lang.Get(langName)
	if err != nil {
		return "", err
	}

	duckdbVer, rtVer, _ := resolveVersions(l)

	// Force local build when --build, -a, or -l are set.
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

	// Use the local image if it already exists.
	if cli.ImageExists(ctx, tag) {
		return tag, nil
	}

	// Try pulling from GHCR.
	remoteRef := remoteImageRef(langName, duckdbVer, rtVer)
	fmt.Fprintf(os.Stderr, "Pulling %s...\n", remoteRef)
	pullErr := cli.Pull(ctx, docker.PullOptions{
		RemoteRef: remoteRef,
		LocalTag:  tag,
	})
	if pullErr == nil {
		return tag, nil
	}

	fmt.Fprintf(os.Stderr, "Pull failed (%v), building locally...\n", pullErr)
	return buildImage(ctx, langName)
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

	// Get the embedded filesystem for this language.
	contextFS, err := fs.Sub(dockerfiles.FS, l.Dir)
	if err != nil {
		return "", fmt.Errorf("loading dockerfiles for %s: %w", langName, err)
	}

	duckdbVer, rtVer, libVer := resolveVersions(l)

	buildArgs := make(map[string]string)
	if duckdbVer != "" && l.HasVersionOverride() {
		buildArgs[l.DuckDBVersionArg] = duckdbVer
	}
	if libVer != "" && l.HasLibVersion() {
		buildArgs[l.LibVersionArg] = libVer
	}
	if rtVer != "" && l.HasRuntimeVersion() {
		buildArgs[l.RuntimeVersionArg] = rtVer
	}

	// Platform.
	var platform string
	if flagArch != "" {
		platform = fmt.Sprintf("linux/%s", flagArch)
	}

	tag := imageTag(langName, duckdbVer, rtVer)

	fmt.Fprintf(os.Stderr, "Building %s %s with DuckDB %s...\n", l.Name, rtVer, duckdbVer)
	err = cli.Build(ctx, docker.BuildOptions{
		ContextFS: contextFS,
		Tag:       tag,
		Platform:  platform,
		BuildArgs: buildArgs,
	})
	if err != nil {
		return "", err
	}

	return tag, nil
}

func imageTag(langName, duckdbVer, runtimeVer string) string {
	ver := duckdbVer
	if ver == "" {
		ver = "latest"
	}
	rt := runtimeVer
	if rt == "" {
		rt = "default"
	}
	tag := fmt.Sprintf("puddle-%s:%s-%s", langName, ver, rt)
	if flagArch != "" {
		tag += "-" + flagArch
	}
	return tag
}

func remoteImageRef(langName, duckdbVer, runtimeVer string) string {
	ver := duckdbVer
	if ver == "" {
		ver = "latest"
	}
	rt := runtimeVer
	if rt == "" {
		rt = "default"
	}
	return fmt.Sprintf("%s/puddle-%s:%s-%s", docker.GHCRPrefix, langName, ver, rt)
}
