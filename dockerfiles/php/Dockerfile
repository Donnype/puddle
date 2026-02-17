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

# Detect arch at build time (works without buildx)
RUN ARCH=$(dpkg --print-architecture) && \
    composer init --name=puddle/repl --no-interaction && \
    if [[ $DUCKDB_PHP_VERSION == 2.* ]]; then \
        curl -sL "https://github.com/duckdb/duckdb/releases/download/v${DUCKDB_VERSION}/libduckdb-linux-${ARCH}.zip" -o /tmp/duckdb.zip \
        && unzip /tmp/duckdb.zip && rm /tmp/duckdb.zip \
        && composer require "satur.io/duckdb:v${DUCKDB_PHP_VERSION}" \
        && cp vendor/satur.io/duckdb/header/linux-${ARCH}/duckdb-ffi.h . \
    ; else \
        composer require "satur.io/duckdb:v${DUCKDB_PHP_VERSION}" \
    ; fi

COPY repl.php .
CMD ["rlwrap", "php", "repl.php"]
