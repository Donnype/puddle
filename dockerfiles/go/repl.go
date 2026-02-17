package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"strings"

	_ "github.com/duckdb/duckdb-go/v2"
)

func main() {
	dbPath := ""
	if len(os.Args) > 1 {
		dbPath = os.Args[1]
	}

	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open DuckDB: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	var version string
	if err := db.QueryRow("SELECT version()").Scan(&version); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get version: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("puddle DuckDB %s (Go)\n", version)
	fmt.Println(`Enter ".quit" to exit.`)

	scanner := bufio.NewScanner(os.Stdin)
	var buf []string

	for {
		if len(buf) == 0 {
			fmt.Print("Go:D ")
		} else {
			fmt.Print("Go:.. ")
		}

		if !scanner.Scan() {
			fmt.Println()
			break
		}

		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if len(buf) == 0 && (trimmed == ".quit" || trimmed == ".exit") {
			break
		}

		buf = append(buf, line)
		query := strings.TrimSpace(strings.Join(buf, "\n"))

		if !strings.HasSuffix(query, ";") {
			continue
		}
		buf = nil

		rows, err := db.Query(query)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		cols, _ := rows.Columns()
		if len(cols) > 0 {
			fmt.Println(strings.Join(cols, "\t"))
			values := make([]interface{}, len(cols))
			ptrs := make([]interface{}, len(cols))
			for i := range values {
				ptrs[i] = &values[i]
			}
			for rows.Next() {
				rows.Scan(ptrs...)
				strs := make([]string, len(cols))
				for i, v := range values {
					if v == nil {
						strs[i] = "NULL"
					} else {
						strs[i] = fmt.Sprintf("%v", v)
					}
				}
				fmt.Println(strings.Join(strs, "\t"))
			}
		}
		rows.Close()
	}
}
