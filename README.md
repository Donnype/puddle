# puddle

A CLI tool that spins up a DuckDB SQL REPL with any language binding via Docker. Ships self-contained Dockerfiles for 7 languages, each with an interactive SQL prompt that talks to DuckDB through the language's native binding.

## Requirements

- [Docker](https://docs.docker.com/get-docker/) (for default mode)
- Go 1.24+ (to build puddle itself)

## Install

```
go build -o puddle .
```

## Quick start

```
# Start a Python-backed DuckDB REPL
puddle run python

# Use a specific DuckDB version
puddle run python -d 1.3.0

# Use a specific Python version
puddle run python -r 3.11

# Pass environment variables into the container
puddle run python -e MOTHERDUCK_TOKEN=your_token -e MY_VAR=value
```

Inside the REPL:

```
puddle DuckDB v1.4.4 (Python)
Enter ".quit" to exit.
Python:D SELECT 42 AS answer;
┌────────┐
│ answer │
│ int32  │
├────────┤
│     42 │
└────────┘
Python:D .quit
```

## Supported languages

```
$ puddle list
Available languages:

  go         Go         runtime=1.24, duckdb=1.4.4
  java       Java       runtime=21, duckdb=1.4.4
  node       Node.js    runtime=20, duckdb=1.4.4
  php        PHP        runtime=8.3, duckdb=1.4.4, lib=2.0.3
  python     Python     runtime=3.12, duckdb=1.4.4
  ruby       Ruby       runtime=3, duckdb=1.4.4
  rust       Rust       runtime=1, duckdb=1.4.4
```

All languages support DuckDB versions from 1.2.x to 1.4.4, and both amd64 and arm64 architectures.

## Commands

### `puddle run <language>`

Build the Docker image and start an interactive SQL REPL.

```
puddle run node                       # Node.js with defaults
puddle run java -d 1.3.0 -r 17       # Java 17 with DuckDB 1.3.0
puddle run php -l 2.0.3              # PHP with specific library version
puddle run ruby -a arm64             # Force arm64 architecture
puddle run python -e FOO=bar         # Pass env vars to the container
```

| Flag | Description |
|------|-------------|
| `-d, --duckdb-version` | DuckDB version (default: per-language) |
| `-r, --runtime-version` | Language runtime version (e.g. Python 3.11) |
| `-l, --lib-version` | Language library version (e.g. PHP duckdb lib) |
| `-a, --arch` | Target architecture: `amd64`, `arm64` |
| `-e, --env` | Environment variable `KEY=VALUE` (repeatable) |
| `--native` | Run without Docker, using the host's runtime |
| `--binary` | Override the runtime binary path (native mode) |

### `puddle build <language>`

Build the Docker image without starting the REPL. Accepts the same `-d`, `-r`, `-l`, `-a` flags.

```
puddle build rust -d 1.4.4
```

### `puddle list`

List all available language bindings with their defaults and supported version ranges.

## Native mode

Run the REPL directly on the host without Docker using `--native`. This extracts the embedded REPL script to a temp directory, installs dependencies if needed, and runs it with the host's language runtime.

```
# Python (requires: pip install duckdb)
puddle run python --native

# Node.js (auto-installs @duckdb/node-api via npm)
puddle run node --native

# Use a specific binary
puddle run python --native --binary /usr/bin/python3.11

# Java (auto-downloads JDBC jar and compiles)
puddle run java --native -d 1.4.4
```

| Language | Prerequisites | Auto-setup |
|----------|--------------|------------|
| Python | `pip install duckdb` | None |
| Ruby | `gem install duckdb` + libduckdb | None |
| Node.js | `node`, `npm` | `npm install @duckdb/node-api` |
| Java | `java`, `javac` | Downloads JDBC jar, compiles |
| Go | `go`, C compiler | `go mod tidy`, `go build` |
| Rust | `cargo` | `cargo build --release` |
| PHP | `php`, `composer`, FFI ext | `composer require satur.io/duckdb` |

## MotherDuck

The `MOTHERDUCK_TOKEN` environment variable is automatically forwarded into the container. You can also pass it explicitly:

```
puddle run python -e MOTHERDUCK_TOKEN=your_token
```

## REPL usage

All REPLs share the same interface:

- SQL statements are terminated with `;`
- Multiline input is supported (continuation prompt: `Language:.. `)
- Type `.quit` or `.exit` to leave
- Arrow keys and history work via [rlwrap](https://github.com/hanslub42/rlwrap)
