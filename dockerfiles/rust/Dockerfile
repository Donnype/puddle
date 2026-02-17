ARG RUST_VERSION=1
FROM rust:${RUST_VERSION}-slim-bookworm

ARG DUCKDB_VERSION=1.4.4

WORKDIR /app

RUN apt-get update && apt-get install -y build-essential pkg-config rlwrap && rm -rf /var/lib/apt/lists/*

COPY Cargo.toml ./
RUN mkdir src && echo 'fn main() {}' > src/main.rs
RUN sed -i "s/DUCKDB_VERSION/${DUCKDB_VERSION}/" Cargo.toml && cargo build --release 2>&1 || true
# Rebuild with actual source
COPY src/ src/
RUN touch src/main.rs && cargo build --release
CMD ["rlwrap", "target/release/repl"]
