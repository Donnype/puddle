ARG PYTHON_VERSION=3.12
FROM python:${PYTHON_VERSION}-slim

ARG DUCKDB_VERSION=1.4.4

WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends rlwrap && rm -rf /var/lib/apt/lists/*
RUN pip install --no-cache-dir "duckdb==${DUCKDB_VERSION}"
COPY repl.py .
CMD ["rlwrap", "python3", "repl.py"]
