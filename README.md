# puddle

A CLI tool that spins up DuckDB language containers via Docker. Ships self-contained Dockerfiles for 7 languages, each with DuckDB pre-installed and an interactive SQL REPL.

## Requirements

- [Docker](https://docs.docker.com/get-docker/) (for default mode)
- Go 1.24+ (to build puddle itself)

## Install

```
go build -o puddle .
```

## Quick start

```
# Start a bash shell with DuckDB + Python, PWD mounted at /work
puddle run python

# Start the DuckDB SQL REPL instead
puddle run python --repl

# Use a specific DuckDB version
puddle run python -d 1.3.0

# Use a specific Python version
puddle run python -r 3.11

# Pass environment variables into the container
puddle run python -e MY_VAR=value
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

Pull a pre-built image from GHCR (or build locally) and start a bash shell with the current directory mounted at `/work`. Use `--repl` to start the DuckDB SQL REPL instead.

```
puddle run python                        # bash shell with PWD at /work
puddle run python --repl                 # DuckDB SQL REPL
puddle run java -d 1.3.0 -r 17          # Java 17 with DuckDB 1.3.0
puddle run python -e FOO=bar             # pass env vars to the container
puddle run python -c "SELECT 42;"        # run SQL and exit
```

| Flag | Description |
|------|-------------|
| `--repl` | Start the DuckDB SQL REPL instead of a shell |
| `-d, --duckdb-version` | DuckDB version (default: per-language) |
| `-r, --runtime-version` | Language runtime version (e.g. Python 3.11) |
| `-l, --lib-version` | Language library version (e.g. PHP duckdb lib) |
| `-a, --arch` | Target architecture: `amd64`, `arm64` |
| `-e, --env` | Environment variable `KEY=VALUE` (repeatable) |
| `-c, --command` | Execute a SQL command and exit (use `-` for stdin) |
| `--build` | Force a local build, skip pulling from GHCR |
| `--native` | Run without Docker, using the host's runtime |
| `--binary` | Override the runtime binary path (native mode) |

### `puddle shell <language>`

Shorthand for `puddle run <language>` — starts a bash shell with PWD mounted at `/work`.

```
puddle shell node
puddle shell ruby -d 1.3.0
```

### `puddle build <language>`

Pull a pre-built image from GHCR, or build locally. Accepts the same `-d`, `-r`, `-l`, `-a` flags.
Use `--build` to force a local build and skip pulling from GHCR.

```
puddle build rust -d 1.4.4
puddle build python --build              # skip GHCR, build locally
```

### `puddle clean`

Remove all locally cached puddle images (both local builds and GHCR pulls).

```
puddle clean                             # remove all puddle images
puddle clean -f                          # force removal
```

### `puddle use <language>`

Set default language and versions for the current shell session via environment variables. Wrap in `eval` to apply:

```
eval "$(puddle use python -d 1.3.0 -r 3.11)"
```

Once set, the language argument becomes optional for `run`, `shell`, and `build`:

```
puddle run                               # uses PUDDLE_LANG, PUDDLE_DUCKDB_VERSION, etc.
puddle run -c "SELECT version();"        # same, SQL command mode
puddle run java -d 1.4.4                 # CLI flags override env vars
```

Clear with:

```
eval "$(puddle use --clear)"
```

### `puddle show`

Display the current puddle environment (set by `puddle use`):

```
$ puddle show
Language:        python (Python)
DuckDB version:  1.3.0
Runtime version: 3.11
```

### `puddle list`

List all available language bindings with their defaults and supported version ranges.

## Non-interactive mode

Run SQL without entering the interactive REPL:

```
# Execute a SQL command directly
puddle run python -c "SELECT 42 AS answer;"

# Read SQL from stdin
echo "SELECT version();" | puddle run python -c -

# Run a SQL file
puddle run python query.sql

# Combine with other flags
puddle run java -d 1.3.0 -c "SELECT version();"
```

In non-interactive mode, the REPL suppresses the banner and prompts, printing only query results. SQL without a trailing `;` is executed on EOF.

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
