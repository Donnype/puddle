ARG RUBY_VERSION=3
FROM ruby:${RUBY_VERSION}

ARG DUCKDB_VERSION=1.4.4

WORKDIR /app

RUN apt-get update && apt-get install -y build-essential wget unzip rlwrap && rm -rf /var/lib/apt/lists/*

# Download libduckdb — detect arch at build time (works without buildx)
RUN ARCH=$(dpkg --print-architecture) && \
    (wget -q "https://github.com/duckdb/duckdb/releases/download/v${DUCKDB_VERSION}/libduckdb-linux-${ARCH}.zip" -O duckdb.zip \
     || wget -q "https://github.com/duckdb/duckdb/releases/download/v${DUCKDB_VERSION}/libduckdb-linux-aarch64.zip" -O duckdb.zip) \
    && unzip duckdb.zip -d libduckdb \
    && mkdir -p /opt/local/include /opt/local/lib \
    && cp libduckdb/duckdb.h /opt/local/include/ \
    && (cp libduckdb/duckdb.hpp /opt/local/include/ 2>/dev/null || true) \
    && cp libduckdb/libduckdb.so /opt/local/lib/ \
    && ldconfig /opt/local/lib \
    && rm -rf duckdb.zip libduckdb

# Pin gem to match the DuckDB major.minor so older libduckdb is supported.
RUN gem install duckdb -v "~> $(echo $DUCKDB_VERSION | cut -d. -f1,2).0"

COPY repl.rb .
CMD ["rlwrap", "ruby", "repl.rb"]
