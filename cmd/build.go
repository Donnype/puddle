package cmd

import (
	"context"
	"fmt"
	"io/fs"
	"os"

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

func addBuildFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&flagDuckDBVersion, "duckdb-version", "d", "", "DuckDB version (uses language default if unset)")
	cmd.Flags().StringVarP(&flagArch, "arch", "a", "", "target architecture: amd64, arm64")
	cmd.Flags().StringVarP(&flagLibVersion, "lib-version", "l", "", "language library version (e.g. PHP duckdb lib)")
	cmd.Flags().StringVarP(&flagRuntimeVersion, "runtime-version", "r", "", "language runtime version (e.g. Python 3.11, Java 17)")
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
func resolveVersions(l lang.Language) (duckdbVer, rtVer, libVer string) {
	return resolveVersionsFrom(l, true)
}

// resolveVersionsNoSession resolves versions skipping the current session.
// Used by "puddle use" when creating a new session.
func resolveVersionsNoSession(l lang.Language) (duckdbVer, rtVer, libVer string) {
	return resolveVersionsFrom(l, false)
}

func resolveVersionsFrom(l lang.Language, includeSession bool) (duckdbVer, rtVer, libVer string) {
	var sess config.Config
	if includeSession {
		sess = config.LoadSession()
	}
	global := config.LoadGlobal()

	// DuckDB version.
	duckdbVer = flagDuckDBVersion
	if duckdbVer == "" {
		duckdbVer = sess.DuckDBVersion
	}
	if duckdbVer == "" {
		duckdbVer = global.DuckDBVersion
	}
	if duckdbVer == "" {
		duckdbVer = l.DefaultDuckDB
	}

	// Runtime version.
	rtVer = flagRuntimeVersion
	if rtVer == "" {
		rtVer = sess.RuntimeVersion
	}
	if rtVer == "" {
		rtVer = global.RuntimeVersion
	}
	if rtVer == "" {
		rtVer = l.DefaultRuntime
	}

	// Lib version.
	libVer = flagLibVersion
	if libVer == "" {
		libVer = sess.LibVersion
	}
	if libVer == "" {
		libVer = global.LibVersion
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
