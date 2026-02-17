ARG JAVA_VERSION=21
FROM eclipse-temurin:${JAVA_VERSION}-jdk

ARG DUCKDB_VERSION=1.4.4

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends rlwrap && rm -rf /var/lib/apt/lists/*

RUN MAVEN_URL="https://repo1.maven.org/maven2/org/duckdb/duckdb_jdbc" && \
    (curl -sfL "${MAVEN_URL}/${DUCKDB_VERSION}/duckdb_jdbc-${DUCKDB_VERSION}.jar" -o duckdb_jdbc.jar \
     || curl -sfL "${MAVEN_URL}/${DUCKDB_VERSION}.0/duckdb_jdbc-${DUCKDB_VERSION}.0.jar" -o duckdb_jdbc.jar)

COPY Repl.java .
RUN javac -cp duckdb_jdbc.jar Repl.java
CMD ["rlwrap", "java", "-cp", ".:duckdb_jdbc.jar", "Repl"]
