ARG PHP_VERSION=8.3
FROM php:${PHP_VERSION}-cli

SHELL ["/bin/bash", "-c"]

ARG DUCKDB_VERSION=1.4.4
ARG DUCKDB_PHP_VERSION=2.0.3

WORKDIR /app

RUN apt-get update && apt-get install -y libffi-dev git unzip curl rlwrap && rm -rf /var/lib/apt/lists/*
RUN docker-php-ext-install ffi bcmath

COPY --from=composer:latest /usr/bin/composer /usr/bin/composer

ENV DUCKDB_PHP_PATH=/app

# Detect arch at build time (works without buildx).
# DuckDB < 1.3.0 used "aarch64" in asset names; 1.3.0+ uses "arm64".
# satur.io/duckdb v2.x requires DuckDB >= 1.3.0; fall back to v1.2.1 for older.
RUN ARCH=$(dpkg --print-architecture) && \
    if [[ $DUCKDB_VERSION == 1.2.* ]] && [[ $DUCKDB_PHP_VERSION == 2.* ]]; then \
        DUCKDB_PHP_VERSION="1.2.1"; \
    fi && \
    composer init --name=puddle/repl --no-interaction && \
    if [[ $DUCKDB_PHP_VERSION == 2.* ]]; then \
        (curl -sfL "https://github.com/duckdb/duckdb/releases/download/v${DUCKDB_VERSION}/libduckdb-linux-${ARCH}.zip" -o /tmp/duckdb.zip \
         || curl -sfL "https://github.com/duckdb/duckdb/releases/download/v${DUCKDB_VERSION}/libduckdb-linux-aarch64.zip" -o /tmp/duckdb.zip) \
        && unzip /tmp/duckdb.zip && rm /tmp/duckdb.zip \
        && composer require "satur.io/duckdb:v${DUCKDB_PHP_VERSION}" \
        && (cp vendor/satur.io/duckdb/header/linux-${ARCH}/duckdb-ffi.h . 2>/dev/null \
            || cp vendor/satur.io/duckdb/header/linux-aarch64/duckdb-ffi.h .) \
    ; else \
        composer require "satur.io/duckdb:v${DUCKDB_PHP_VERSION}" \
    ; fi

COPY repl.php .
CMD ["rlwrap", "php", "repl.php"]
