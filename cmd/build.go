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
)

var buildCmd = &cobra.Command{
	Use:   "build <language>",
	Short: "Build a Docker image for a language binding",
	Args:  cobra.ExactArgs(1),
	RunE:  runBuild,
}

func init() {
	addBuildFlags(buildCmd)
}

func addBuildFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&flagDuckDBVersion, "duckdb-version", "d", "", "DuckDB version (uses language default if unset)")
	cmd.Flags().StringVarP(&flagArch, "arch", "a", "", "target architecture: amd64, arm64")
	cmd.Flags().StringVarP(&flagLibVersion, "lib-version", "l", "", "language library version (e.g. PHP duckdb lib)")
	cmd.Flags().StringVarP(&flagRuntimeVersion, "runtime-version", "r", "", "language runtime version (e.g. Python 3.11, Java 17)")
}

func runBuild(cmd *cobra.Command, args []string) error {
	tag, err := buildImage(cmd.Context(), args[0])
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "\nImage ready: %s\n", tag)
	return nil
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

	buildArgs := make(map[string]string)

	// DuckDB version.
	duckdbVer := flagDuckDBVersion
	if duckdbVer == "" {
		duckdbVer = l.DefaultDuckDB
	}
	if duckdbVer != "" && l.HasVersionOverride() {
		buildArgs[l.DuckDBVersionArg] = duckdbVer
	}

	// Library version.
	libVer := flagLibVersion
	if libVer == "" {
		libVer = l.DefaultLib
	}
	if libVer != "" && l.HasLibVersion() {
		buildArgs[l.LibVersionArg] = libVer
	}

	// Runtime version.
	rtVer := flagRuntimeVersion
	if rtVer == "" {
		rtVer = l.DefaultRuntime
	}
	if rtVer != "" && l.HasRuntimeVersion() {
		buildArgs[l.RuntimeVersionArg] = rtVer
	}

	// Platform.
	var platform string
	if flagArch != "" {
		platform = fmt.Sprintf("linux/%s", flagArch)
	}

	tag := imageTag(langName, duckdbVer)

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

func imageTag(langName, duckdbVer string) string {
	ver := duckdbVer
	if ver == "" {
		ver = "latest"
	}
	tag := fmt.Sprintf("puddle-%s:%s", langName, ver)
	if flagArch != "" {
		tag += "-" + flagArch
	}
	return tag
}
