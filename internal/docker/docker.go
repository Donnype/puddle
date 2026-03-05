package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/term"
)

// GHCRPrefix is the GHCR registry prefix for pre-built puddle images.
const GHCRPrefix = "ghcr.io/donnype"

// RunOptions configures a Docker container run.
type RunOptions struct {
	Image      string    // image tag
	Env        []string  // environment variables (KEY=VALUE)
	Stdin      io.Reader // if non-nil, pipe this as stdin (batch mode)
	Binds      []string  // volume mounts, e.g. ["/host/path:/container/path"]
	WorkingDir string    // container working directory
	Cmd        []string  // override CMD (e.g. ["/bin/bash"])
}

// PullOptions configures a Docker image pull.
type PullOptions struct {
	RemoteRef string // e.g. "ghcr.io/donnype/puddle-python:1.4.4-3.12"
	LocalTag  string // local tag to apply after pull (empty = skip retag)
}

// ImageInfo holds basic info about a local Docker image.
type ImageInfo struct {
	ID   string
	Tags []string
}

// Ping checks that the Docker daemon is reachable.
func Ping(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker", "info")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cannot reach Docker daemon: %w\n(is Docker running?)", err)
	}
	return nil
}

// Run creates, starts, and attaches to a container.
// When opts.Stdin is nil, the container runs interactively with a TTY.
// When opts.Stdin is set, input is piped and output is copied to stdout/stderr.
func Run(ctx context.Context, opts RunOptions) error {
	args := []string{"run", "--rm"}

	if opts.Stdin != nil {
		args = append(args, "-i")
	} else if term.IsTerminal(int(os.Stdin.Fd())) {
		args = append(args, "-it")
	} else {
		args = append(args, "-i")
	}

	for _, e := range opts.Env {
		args = append(args, "-e", e)
	}
	for _, b := range opts.Binds {
		args = append(args, "-v", b)
	}
	if opts.WorkingDir != "" {
		args = append(args, "-w", opts.WorkingDir)
	}

	args = append(args, opts.Image)
	args = append(args, opts.Cmd...)

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if opts.Stdin != nil {
		cmd.Stdin = opts.Stdin
	} else {
		cmd.Stdin = os.Stdin
	}

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("container exited with status %d", exitErr.ExitCode())
		}
		return err
	}
	return nil
}

// Pull pulls a remote image and optionally retags it to a local name.
func Pull(ctx context.Context, opts PullOptions) error {
	cmd := exec.CommandContext(ctx, "docker", "pull", opts.RemoteRef)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pulling %s: %w", opts.RemoteRef, err)
	}

	if opts.LocalTag != "" {
		tag := exec.CommandContext(ctx, "docker", "tag", opts.RemoteRef, opts.LocalTag)
		if err := tag.Run(); err != nil {
			return fmt.Errorf("tagging %s as %s: %w", opts.RemoteRef, opts.LocalTag, err)
		}
	}
	return nil
}

// ImageExists checks if a local image with the given tag exists.
func ImageExists(ctx context.Context, tag string) bool {
	cmd := exec.CommandContext(ctx, "docker", "image", "inspect", tag)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run() == nil
}

// ListImages returns all local images with tags matching "puddle-*".
func ListImages(ctx context.Context) ([]ImageInfo, error) {
	refs := []string{"puddle-*", GHCRPrefix + "/puddle-*"}
	seen := make(map[string]*ImageInfo)
	var result []ImageInfo

	for _, ref := range refs {
		cmd := exec.CommandContext(ctx, "docker", "images",
			"--filter", "reference="+ref,
			"--format", "{{json .}}")
		out, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("listing images: %w", err)
		}

		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if line == "" {
				continue
			}
			var img struct {
				ID         string `json:"ID"`
				Repository string `json:"Repository"`
				Tag        string `json:"Tag"`
			}
			if err := json.Unmarshal([]byte(line), &img); err != nil {
				continue
			}
			fullTag := img.Repository + ":" + img.Tag
			if existing, ok := seen[img.ID]; ok {
				existing.Tags = append(existing.Tags, fullTag)
			} else {
				info := &ImageInfo{ID: img.ID, Tags: []string{fullTag}}
				seen[img.ID] = info
				result = append(result, *info)
			}
		}
	}

	// Fix: result entries are copies, rebuild from map.
	result = result[:0]
	for _, info := range seen {
		result = append(result, *info)
	}

	return result, nil
}

// RemoveImage removes a Docker image by ID.
func RemoveImage(ctx context.Context, id string, force bool) error {
	args := []string{"rmi", id}
	if force {
		args = []string{"rmi", "-f", id}
	}
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("removing image %s: %w", id, err)
	}
	return nil
}
