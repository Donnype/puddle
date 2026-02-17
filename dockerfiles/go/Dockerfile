ARG GO_VERSION=1.24
FROM golang:${GO_VERSION}-bookworm

ARG DUCKDB_VERSION=1.4.4

WORKDIR /app

RUN apt-get update && apt-get install -y gcc g++ rlwrap && rm -rf /var/lib/apt/lists/*

COPY go.mod.tmpl repl.go ./
RUN mv go.mod.tmpl go.mod

# Map DuckDB version to duckdb-go SDK version and module path.
# v2.5.0+ lives at github.com/duckdb/duckdb-go/v2 (new home).
# v2.4.0 and earlier at github.com/marcboeker/go-duckdb/v2 (original).
RUN case "$DUCKDB_VERSION" in \
        "1.4.4") SDK_VERSION="v2.5.5" ;; \
        "1.4.3") SDK_VERSION="v2.5.4" ;; \
        "1.4.2") SDK_VERSION="v2.5.3" ;; \
        "1.4.1") SDK_VERSION="v2.5.1" ;; \
        "1.4.0") SDK_VERSION="v2.5.0" ;; \
        "1.3."*) SDK_VERSION="v2.4.0" ;; \
        "1.2."*) SDK_VERSION="v2.3.0" ;; \
        *) echo "Unsupported DuckDB version for Go: $DUCKDB_VERSION (add mapping to Dockerfile)" && exit 1 ;; \
    esac \
    && case "$SDK_VERSION" in \
        v2.[0-4].*) MODULE_PATH="github.com/marcboeker/go-duckdb/v2" ;; \
        *)          MODULE_PATH="github.com/duckdb/duckdb-go/v2" ;; \
    esac \
    && sed -i "s|MODULE_PATH_PLACEHOLDER|${MODULE_PATH}|" go.mod repl.go \
    && sed -i "s|SDK_VERSION_PLACEHOLDER|${SDK_VERSION}|" go.mod \
    && go mod tidy

RUN CGO_ENABLED=1 go build -o /repl .
CMD ["rlwrap", "/repl"]
