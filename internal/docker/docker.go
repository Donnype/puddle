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

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"golang.org/x/term"
)

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
	Image string    // image tag
	Env   []string  // environment variables (KEY=VALUE)
	Stdin io.Reader // if non-nil, pipe this as stdin (non-interactive, no TTY)
}

// Run creates, starts, and attaches to a container.
// When opts.Stdin is nil, the container runs interactively with a TTY.
// When opts.Stdin is set, input is piped without a TTY (batch mode).
func (c *Client) Run(ctx context.Context, opts RunOptions) error {
	interactive := opts.Stdin == nil

	config := &container.Config{
		Image:     opts.Image,
		Env:       opts.Env,
		Tty:       interactive,
		OpenStdin: true,
	}

	hostConfig := &container.HostConfig{}
	if interactive {
		// Always set a valid console size — rlwrap refuses to start with 0x0.
		fd := int(os.Stdin.Fd())
		h, w := 24, 80
		if term.IsTerminal(fd) {
			if tw, th, err := term.GetSize(fd); err == nil {
				h, w = th, tw
			}
		}
		hostConfig.ConsoleSize = [2]uint{uint(h), uint(w)}
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

	// Set terminal to raw mode for interactive I/O.
	if interactive {
		fd := int(os.Stdin.Fd())
		if term.IsTerminal(fd) {
			oldState, err := term.MakeRaw(fd)
			if err == nil {
				defer term.Restore(fd, oldState)
			}
		}
	}

	// container -> stdout (+ stderr in non-TTY mode).
	outputDone := make(chan error, 1)
	go func() {
		if interactive {
			// TTY mode: raw stream, stdout only.
			_, err := io.Copy(os.Stdout, attachResp.Reader)
			outputDone <- err
		} else {
			// Non-TTY mode: multiplexed stream, demux stdout/stderr.
			_, err := stdcopy.StdCopy(os.Stdout, os.Stderr, attachResp.Reader)
			outputDone <- err
		}
	}()

	// stdin -> container.
	stdinSource := io.Reader(os.Stdin)
	if opts.Stdin != nil {
		stdinSource = opts.Stdin
	}
	go func() {
		io.Copy(attachResp.Conn, stdinSource)
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
