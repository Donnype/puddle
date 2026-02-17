package cmd

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"puddle/dockerfiles"
	"puddle/internal/lang"
)

// runNative extracts the embedded REPL files and runs them with the host's runtime.
// When sqlReader is non-nil, SQL is piped in without rlwrap (batch mode).
func runNative(ctx context.Context, langName, binary string, sqlReader io.Reader) error {
	l, err := lang.Get(langName)
	if err != nil {
		return err
	}

	tmpDir, err := os.MkdirTemp("", "puddle-native-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	langFS, err := fs.Sub(dockerfiles.FS, l.Dir)
	if err != nil {
		return fmt.Errorf("loading files for %s: %w", l.Name, err)
	}
	if err := extractFS(langFS, tmpDir); err != nil {
		return fmt.Errorf("extracting files: %w", err)
	}

	if binary == "" {
		binary = l.NativeBin
	}

	duckdbVer := flagDuckDBVersion
	if duckdbVer == "" {
		duckdbVer = l.DefaultDuckDB
	}

	cmdArgs, err := nativeSetup(ctx, l, tmpDir, binary, duckdbVer)
	if err != nil {
		return err
	}

	// Only use rlwrap in interactive mode.
	if sqlReader == nil {
		if path, lookErr := exec.LookPath("rlwrap"); lookErr == nil {
			cmdArgs = append([]string{path}, cmdArgs...)
		}
	}

	cmd := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)
	cmd.Dir = tmpDir
	if sqlReader != nil {
		cmd.Stdin = sqlReader
	} else {
		cmd.Stdin = os.Stdin
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if len(flagEnv) > 0 {
		cmd.Env = append(os.Environ(), flagEnv...)
	}

	if sqlReader == nil {
		fmt.Fprintf(os.Stderr, "\nStarting REPL (native)...\n")
	}
	return cmd.Run()
}

// extractFS copies all files from an embedded fs.FS to a directory, skipping Dockerfiles.
func extractFS(src fs.FS, dest string) error {
	return fs.WalkDir(src, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != "." {
				return os.MkdirAll(filepath.Join(dest, path), 0755)
			}
			return nil
		}
		if strings.EqualFold(filepath.Base(path), "dockerfile") {
			return nil
		}
		data, err := fs.ReadFile(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, path)
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0644)
	})
}

// nativeSetup performs per-language setup and returns the command to run the REPL.
func nativeSetup(ctx context.Context, l lang.Language, dir, binary, duckdbVer string) ([]string, error) {
	switch l.Dir {
	case "python":
		return []string{binary, "repl.py"}, nil
	case "ruby":
		return []string{binary, "repl.rb"}, nil
	case "node":
		return setupNode(ctx, dir, binary, duckdbVer)
	case "java":
		return setupJava(ctx, dir, binary, duckdbVer)
	case "go":
		return setupGo(ctx, dir, binary, duckdbVer)
	case "rust":
		return setupRust(ctx, dir, binary, duckdbVer)
	case "php":
		return setupPHP(ctx, dir, binary, duckdbVer)
	default:
		return nil, fmt.Errorf("native mode not supported for %s", l.Name)
	}
}

func setupNode(ctx context.Context, dir, binary, duckdbVer string) ([]string, error) {
	fmt.Fprintf(os.Stderr, "Installing @duckdb/node-api...\n")

	npmRun := func(args ...string) error {
		cmd := exec.CommandContext(ctx, "npm", args...)
		cmd.Dir = dir
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	if err := npmRun("init", "-y"); err != nil {
		return nil, fmt.Errorf("npm init: %w", err)
	}

	// Try exact version, then tilde, then latest (same strategy as Dockerfile).
	pkg := "@duckdb/node-api"
	if npmRun("install", "--save", fmt.Sprintf("%s@%s", pkg, duckdbVer)) != nil {
		if npmRun("install", "--save", fmt.Sprintf("%s@~%s", pkg, duckdbVer)) != nil {
			if err := npmRun("install", "--save", pkg); err != nil {
				return nil, fmt.Errorf("npm install @duckdb/node-api: %w", err)
			}
		}
	}

	return []string{binary, "repl.mjs"}, nil
}

func setupJava(ctx context.Context, dir, binary, duckdbVer string) ([]string, error) {
	jarName := "duckdb_jdbc.jar"
	jarPath := filepath.Join(dir, jarName)

	fmt.Fprintf(os.Stderr, "Downloading DuckDB JDBC %s...\n", duckdbVer)
	urls := []string{
		fmt.Sprintf("https://repo1.maven.org/maven2/org/duckdb/duckdb_jdbc/%s/duckdb_jdbc-%s.jar", duckdbVer, duckdbVer),
		fmt.Sprintf("https://repo1.maven.org/maven2/org/duckdb/duckdb_jdbc/%s.0/duckdb_jdbc-%s.0.jar", duckdbVer, duckdbVer),
	}
	if err := downloadFirst(urls, jarPath); err != nil {
		return nil, fmt.Errorf("downloading JDBC jar for %s: %w", duckdbVer, err)
	}

	// Derive javac from the java binary path.
	javac := "javac"
	if binary != "java" {
		javac = filepath.Join(filepath.Dir(binary), "javac")
	}

	fmt.Fprintf(os.Stderr, "Compiling Repl.java...\n")
	cmd := exec.CommandContext(ctx, javac, "-cp", jarName, "Repl.java")
	cmd.Dir = dir
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("javac: %w", err)
	}

	return []string{binary, "-cp", ".:" + jarName, "Repl"}, nil
}

var goSDKVersions = map[string]string{
	"1.4.4": "v2.5.5",
	"1.4.3": "v2.5.4",
	"1.4.2": "v2.5.3",
	"1.4.1": "v2.5.1",
	"1.4.0": "v2.5.0",
}

func goSDKVersion(duckdbVer string) (string, error) {
	if v, ok := goSDKVersions[duckdbVer]; ok {
		return v, nil
	}
	if strings.HasPrefix(duckdbVer, "1.3.") {
		return "v2.4.0", nil
	}
	if strings.HasPrefix(duckdbVer, "1.2.") {
		return "v2.3.0", nil
	}
	return "", fmt.Errorf("unsupported DuckDB version for Go: %s (add mapping)", duckdbVer)
}

func goModulePath(sdkVer string) string {
	// v2.5.0+ moved to github.com/duckdb/duckdb-go/v2.
	// v2.4.0 and earlier used github.com/marcboeker/go-duckdb/v2.
	if sdkVer >= "v2.5.0" {
		return "github.com/duckdb/duckdb-go/v2"
	}
	return "github.com/marcboeker/go-duckdb/v2"
}

func setupGo(ctx context.Context, dir, binary, duckdbVer string) ([]string, error) {
	sdkVer, err := goSDKVersion(duckdbVer)
	if err != nil {
		return nil, err
	}

	modPath := goModulePath(sdkVer)

	// Rename go.mod.tmpl to go.mod with version and module path substituted.
	tmplPath := filepath.Join(dir, "go.mod.tmpl")
	data, err := os.ReadFile(tmplPath)
	if err != nil {
		return nil, fmt.Errorf("reading go.mod.tmpl: %w", err)
	}
	modContent := strings.ReplaceAll(string(data), "MODULE_PATH_PLACEHOLDER", modPath)
	modContent = strings.ReplaceAll(modContent, "SDK_VERSION_PLACEHOLDER", sdkVer)
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(modContent), 0644); err != nil {
		return nil, fmt.Errorf("writing go.mod: %w", err)
	}

	// Substitute the module path in repl.go too.
	replPath := filepath.Join(dir, "repl.go")
	replData, err := os.ReadFile(replPath)
	if err != nil {
		return nil, fmt.Errorf("reading repl.go: %w", err)
	}
	replContent := strings.ReplaceAll(string(replData), "MODULE_PATH_PLACEHOLDER", modPath)
	if err := os.WriteFile(replPath, []byte(replContent), 0644); err != nil {
		return nil, fmt.Errorf("writing repl.go: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Running go mod tidy...\n")
	cmd := exec.CommandContext(ctx, binary, "mod", "tidy")
	cmd.Dir = dir
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("go mod tidy: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Building Go REPL...\n")
	replBin := filepath.Join(dir, "repl")
	cmd = exec.CommandContext(ctx, binary, "build", "-o", replBin, ".")
	cmd.Dir = dir
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1")
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("go build: %w", err)
	}

	return []string{replBin}, nil
}

func setupRust(ctx context.Context, dir, binary, duckdbVer string) ([]string, error) {
	// Substitute DUCKDB_VERSION in Cargo.toml.
	cargoPath := filepath.Join(dir, "Cargo.toml")
	data, err := os.ReadFile(cargoPath)
	if err != nil {
		return nil, fmt.Errorf("reading Cargo.toml: %w", err)
	}
	content := strings.ReplaceAll(string(data), "DUCKDB_VERSION", duckdbVer)
	if err := os.WriteFile(cargoPath, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("writing Cargo.toml: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Building Rust REPL (this may take a while)...\n")
	cmd := exec.CommandContext(ctx, binary, "build", "--release")
	cmd.Dir = dir
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("cargo build: %w", err)
	}

	return []string{filepath.Join(dir, "target", "release", "repl")}, nil
}

func setupPHP(ctx context.Context, dir, binary, duckdbVer string) ([]string, error) {
	libVer := flagLibVersion
	if libVer == "" {
		libVer = "2.0.3"
	}

	fmt.Fprintf(os.Stderr, "Setting up PHP project...\n")

	composerRun := func(args ...string) error {
		cmd := exec.CommandContext(ctx, "composer", args...)
		cmd.Dir = dir
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	if err := composerRun("init", "--name=puddle/repl", "--no-interaction"); err != nil {
		return nil, fmt.Errorf("composer init: %w", err)
	}

	pkg := fmt.Sprintf("satur.io/duckdb:v%s", libVer)
	if err := composerRun("require", pkg); err != nil {
		return nil, fmt.Errorf("composer require: %w", err)
	}

	return []string{binary, "repl.php"}, nil
}

// downloadFirst tries each URL in order and saves the first successful download to dest.
func downloadFirst(urls []string, dest string) error {
	for _, url := range urls {
		if err := downloadFile(url, dest); err == nil {
			return nil
		}
	}
	return fmt.Errorf("all download URLs failed")
}

func downloadFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, resp.Body)
	closeErr := f.Close()
	if err != nil {
		os.Remove(dest)
		return err
	}
	if closeErr != nil {
		os.Remove(dest)
		return closeErr
	}
	return nil
}
