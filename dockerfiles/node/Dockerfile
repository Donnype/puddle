ARG NODE_VERSION=20
FROM node:${NODE_VERSION}

ARG DUCKDB_VERSION=1.4.4

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends rlwrap && rm -rf /var/lib/apt/lists/*

COPY repl.mjs .
RUN echo '{"type":"module"}' > package.json && \
    (npm install "@duckdb/node-api@${DUCKDB_VERSION}" 2>/dev/null \
     || npm install "@duckdb/node-api@~${DUCKDB_VERSION}-0" 2>/dev/null \
     || (echo "Could not find @duckdb/node-api for DuckDB ${DUCKDB_VERSION}" && exit 1))

CMD ["rlwrap", "node", "repl.mjs"]
