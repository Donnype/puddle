import duckdb
import sys

def main():
    path = sys.argv[1] if len(sys.argv) > 1 else ":memory:"
    conn = duckdb.connect(path)
    version = conn.execute("SELECT version()").fetchone()[0]
    print(f"puddle DuckDB {version} (Python)")
    print('Enter ".quit" to exit.')

    buf = []
    while True:
        prompt = "Python:D " if not buf else "Python:.. "
        try:
            line = input(prompt)
        except (EOFError, KeyboardInterrupt):
            print()
            break

        trimmed = line.strip()
        if not buf and trimmed in (".quit", ".exit"):
            break

        buf.append(line)
        sql = "\n".join(buf).strip()

        if not sql.endswith(";"):
            continue
        buf = []

        try:
            result = conn.sql(sql)
            if result:
                result.show()
        except Exception as e:
            print(f"Error: {e}")

if __name__ == "__main__":
    main()
