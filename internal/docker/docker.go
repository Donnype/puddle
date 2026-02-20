package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"golang.org/x/term"
)

// GHCRPrefix is the GHCR registry prefix for pre-built puddle images.
const GHCRPrefix = "ghcr.io/donnype"

// Client wraps the Docker Engine API client.
type Client struct {
	cli *client.Client
}

// New creates a Docker client from the environment.
func New() (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("cannot connect to Docker: %w\n(is Docker running?)", err)
	}
	return &Client{cli: cli}, nil
}

// Close releases the Docker client resources.
func (c *Client) Close() error {
	return c.cli.Close()
}

// Ping checks that the Docker daemon is reachable.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.cli.Ping(ctx)
	if err != nil {
		return fmt.Errorf("cannot reach Docker daemon: %w\n(is Docker running?)", err)
	}
	return nil
}

// BuildOptions configures a Docker image build.
type BuildOptions struct {
	ContextFS fs.FS             // filesystem with Dockerfile + context files
	Tag       string            // image tag
	Platform  string            // e.g. "linux/amd64" (empty = host default)
	BuildArgs map[string]string // --build-arg values
}

// Build builds a Docker image from an embedded filesystem.
func (c *Client) Build(ctx context.Context, opts BuildOptions) error {
	tarBuf, err := createTar(opts.ContextFS)
	if err != nil {
		return fmt.Errorf("creating build context: %w", err)
	}

	buildArgs := make(map[string]*string, len(opts.BuildArgs))
	for k, v := range opts.BuildArgs {
		v := v
		buildArgs[k] = &v
	}

	buildOpts := types.ImageBuildOptions{
		Tags:       []string{opts.Tag},
		BuildArgs:  buildArgs,
		Dockerfile: "Dockerfile",
		Remove:     true,
	}
	if opts.Platform != "" {
		buildOpts.Platform = opts.Platform
	}

	resp, err := c.cli.ImageBuild(ctx, tarBuf, buildOpts)
	if err != nil {
		return fmt.Errorf("docker build: %w", err)
	}
	defer resp.Body.Close()

	return streamBuildOutput(resp.Body)
}

// RunOptions configures a Docker container run.
type RunOptions struct {
	Image      string    // image tag
	Env        []string  // environment variables (KEY=VALUE)
	Stdin      io.Reader // if non-nil, pipe this as stdin (batch mode)
	Binds      []string  // volume mounts, e.g. ["/host/path:/container/path"]
	WorkingDir string    // container working directory
	Cmd        []string  // override CMD (e.g. ["/bin/bash"])
}

// Run creates, starts, and attaches to a container.
// When opts.Stdin is nil, the container runs interactively with a TTY.
// When opts.Stdin is set, input is piped and output is read from logs after exit.
func (c *Client) Run(ctx context.Context, opts RunOptions) error {
	if opts.Stdin != nil {
		return c.runBatch(ctx, opts)
	}
	return c.runInteractive(ctx, opts)
}

// runInteractive runs a container with a TTY and bidirectional I/O.
func (c *Client) runInteractive(ctx context.Context, opts RunOptions) error {
	config := &container.Config{
		Image:     opts.Image,
		Env:       opts.Env,
		Tty:       true,
		OpenStdin: true,
	}
	if len(opts.Cmd) > 0 {
		config.Cmd = opts.Cmd
	}
	if opts.WorkingDir != "" {
		config.WorkingDir = opts.WorkingDir
	}

	// Always set a valid console size — rlwrap refuses to start with 0x0.
	fd := int(os.Stdin.Fd())
	h, w := 24, 80
	if term.IsTerminal(fd) {
		if tw, th, err := term.GetSize(fd); err == nil {
			h, w = th, tw
		}
	}
	hostConfig := &container.HostConfig{
		ConsoleSize: [2]uint{uint(h), uint(w)},
		Binds:       opts.Binds,
	}

	resp, err := c.cli.ContainerCreate(ctx, config, hostConfig, nil, nil, "")
	if err != nil {
		return fmt.Errorf("creating container: %w", err)
	}
	containerID := resp.ID
	defer c.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})

	attachResp, err := c.cli.ContainerAttach(ctx, containerID, container.AttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		return fmt.Errorf("attaching to container: %w", err)
	}
	defer attachResp.Close()

	if err := c.cli.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return fmt.Errorf("starting container: %w", err)
	}

	// Set terminal to raw mode for proper interactive I/O.
	if term.IsTerminal(fd) {
		oldState, err := term.MakeRaw(fd)
		if err == nil {
			defer term.Restore(fd, oldState)
		}
	}

	// Bidirectional I/O: stdin -> container, container -> stdout.
	outputDone := make(chan error, 1)
	go func() {
		_, err := io.Copy(os.Stdout, attachResp.Reader)
		outputDone <- err
	}()

	go func() {
		io.Copy(attachResp.Conn, os.Stdin)
		attachResp.CloseWrite()
	}()

	// Wait for the container to exit.
	statusCh, errCh := c.cli.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("waiting for container: %w", err)
		}
	case status := <-statusCh:
		<-outputDone // drain remaining output
		if status.StatusCode != 0 {
			return fmt.Errorf("container exited with status %d", status.StatusCode)
		}
	}

	return nil
}

// runBatch pipes stdin to the container and reads output from logs after exit.
func (c *Client) runBatch(ctx context.Context, opts RunOptions) error {
	config := &container.Config{
		Image:        opts.Image,
		Env:          opts.Env,
		Tty:          false,
		OpenStdin:    true,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
	}
	if len(opts.Cmd) > 0 {
		config.Cmd = opts.Cmd
	}
	if opts.WorkingDir != "" {
		config.WorkingDir = opts.WorkingDir
	}

	resp, err := c.cli.ContainerCreate(ctx, config, &container.HostConfig{Binds: opts.Binds}, nil, nil, "")
	if err != nil {
		return fmt.Errorf("creating container: %w", err)
	}
	containerID := resp.ID
	defer c.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})

	// Attach for stdin only — we'll read output from logs after exit.
	attachResp, err := c.cli.ContainerAttach(ctx, containerID, container.AttachOptions{
		Stream: true,
		Stdin:  true,
	})
	if err != nil {
		return fmt.Errorf("attaching to container: %w", err)
	}

	if err := c.cli.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		attachResp.Close()
		return fmt.Errorf("starting container: %w", err)
	}

	// Pipe SQL into the container's stdin.
	io.Copy(attachResp.Conn, opts.Stdin)
	attachResp.CloseWrite()
	attachResp.Close()

	// Wait for the container to exit.
	statusCh, errCh := c.cli.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("waiting for container: %w", err)
		}
	case status := <-statusCh:
		// Read container logs (contains all stdout/stderr output).
		logReader, logErr := c.cli.ContainerLogs(ctx, containerID, container.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
		})
		if logErr != nil {
			return fmt.Errorf("reading container logs: %w", logErr)
		}
		defer logReader.Close()
		// Non-TTY logs use the Docker multiplexed format.
		stdcopy.StdCopy(os.Stdout, os.Stderr, logReader)

		if status.StatusCode != 0 {
			return fmt.Errorf("container exited with status %d", status.StatusCode)
		}
	}

	return nil
}

// createTar creates a tar archive from an fs.FS.
func createTar(files fs.FS) (*bytes.Buffer, error) {
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)

	err := fs.WalkDir(files, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		data, err := fs.ReadFile(files, path)
		if err != nil {
			return err
		}

		hdr := &tar.Header{
			Name: path,
			Mode: 0644,
			Size: int64(len(data)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		_, err = tw.Write(data)
		return err
	})
	if err != nil {
		return nil, err
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}
	return buf, nil
}

// PullOptions configures a Docker image pull.
type PullOptions struct {
	RemoteRef string // e.g. "ghcr.io/donnype/puddle-python:1.4.4-3.12"
	LocalTag  string // local tag to apply after pull (empty = skip retag)
	Platform  string // e.g. "linux/amd64" (empty = host default)
}

// Pull pulls a remote image and optionally retags it to a local name.
func (c *Client) Pull(ctx context.Context, opts PullOptions) error {
	pullOpts := image.PullOptions{
		Platform: opts.Platform,
	}

	reader, err := c.cli.ImagePull(ctx, opts.RemoteRef, pullOpts)
	if err != nil {
		return fmt.Errorf("pulling %s: %w", opts.RemoteRef, err)
	}
	defer reader.Close()

	if err := streamPullOutput(reader); err != nil {
		return err
	}

	if opts.LocalTag != "" {
		if err := c.cli.ImageTag(ctx, opts.RemoteRef, opts.LocalTag); err != nil {
			return fmt.Errorf("tagging %s as %s: %w", opts.RemoteRef, opts.LocalTag, err)
		}
	}
	return nil
}

// ImageExists checks if a local image with the given tag exists.
func (c *Client) ImageExists(ctx context.Context, tag string) bool {
	_, _, err := c.cli.ImageInspectWithRaw(ctx, tag)
	return err == nil
}

// ImageInfo holds basic info about a local Docker image.
type ImageInfo struct {
	ID   string
	Tags []string
}

// ListImages returns all local images with tags matching "puddle-*".
func (c *Client) ListImages(ctx context.Context) ([]ImageInfo, error) {
	listOpts := image.ListOptions{
		Filters: filters.NewArgs(filters.Arg("reference", "puddle-*")),
	}
	images, err := c.cli.ImageList(ctx, listOpts)
	if err != nil {
		return nil, fmt.Errorf("listing images: %w", err)
	}

	// Also find GHCR-prefixed puddle images.
	ghcrOpts := image.ListOptions{
		Filters: filters.NewArgs(filters.Arg("reference", GHCRPrefix+"/puddle-*")),
	}
	ghcrImages, err := c.cli.ImageList(ctx, ghcrOpts)
	if err != nil {
		return nil, fmt.Errorf("listing GHCR images: %w", err)
	}

	// Merge, deduplicating by ID.
	seen := make(map[string]bool)
	var result []ImageInfo
	for _, img := range append(images, ghcrImages...) {
		if seen[img.ID] {
			// Merge tags into existing entry.
			for i := range result {
				if result[i].ID == img.ID {
					result[i].Tags = append(result[i].Tags, img.RepoTags...)
					break
				}
			}
			continue
		}
		seen[img.ID] = true
		result = append(result, ImageInfo{
			ID:   img.ID,
			Tags: img.RepoTags,
		})
	}
	return result, nil
}

// RemoveImage removes a Docker image by ID.
func (c *Client) RemoveImage(ctx context.Context, id string, force bool) error {
	_, err := c.cli.ImageRemove(ctx, id, image.RemoveOptions{
		Force:         force,
		PruneChildren: true,
	})
	if err != nil {
		return fmt.Errorf("removing image %s: %w", shortID(id), err)
	}
	return nil
}

func shortID(id string) string {
	id = strings.TrimPrefix(id, "sha256:")
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

// streamBuildOutput reads Docker build JSON output and prints it.
func streamBuildOutput(r io.Reader) error {
	decoder := json.NewDecoder(r)
	for {
		var msg struct {
			Stream string `json:"stream"`
			Error  string `json:"error"`
		}
		if err := decoder.Decode(&msg); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if msg.Error != "" {
			return fmt.Errorf("build error: %s", msg.Error)
		}
		if msg.Stream != "" {
			fmt.Print(msg.Stream)
		}
	}
}

// streamPullOutput reads Docker pull JSON output and streams progress to stderr.
func streamPullOutput(r io.Reader) error {
	decoder := json.NewDecoder(r)
	for {
		var msg struct {
			Status string `json:"status"`
			Error  string `json:"error"`
		}
		if err := decoder.Decode(&msg); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if msg.Error != "" {
			return fmt.Errorf("pull error: %s", msg.Error)
		}
		if msg.Status != "" {
			fmt.Fprintln(os.Stderr, msg.Status)
		}
	}
}
