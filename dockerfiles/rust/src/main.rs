use duckdb::Connection;
use std::io::{self, BufRead, Write};

fn main() {
    let conn = Connection::open_in_memory().expect("Failed to open DuckDB");

    let version: String = conn
        .query_row("SELECT version()", [], |row| row.get(0))
        .expect("Failed to get version");
    println!("puddle DuckDB {} (Rust)", version);
    println!("Enter \".quit\" to exit.");

    let stdin = io::stdin();
    let mut buf = String::new();

    loop {
        if buf.is_empty() {
            print!("Rust:D ");
        } else {
            print!("Rust:.. ");
        }
        io::stdout().flush().unwrap();

        let mut line = String::new();
        if stdin.lock().read_line(&mut line).unwrap() == 0 {
            println!();
            break;
        }

        let trimmed = line.trim();
        if buf.is_empty() && (trimmed == ".quit" || trimmed == ".exit") {
            break;
        }

        buf.push_str(&line);
        if !buf.trim().ends_with(';') {
            continue;
        }
        let sql = buf.trim().to_string();
        buf.clear();

        match conn.prepare(&sql) {
            Ok(mut stmt) => {
                let names: Vec<String> =
                    stmt.column_names().iter().map(|s| s.to_string()).collect();
                let ncols = names.len();
                if ncols == 0 {
                    if let Err(e) = stmt.execute([]) {
                        println!("Error: {}", e);
                    }
                    continue;
                }

                println!("{}", names.join("\t"));
                match stmt.query([]) {
                    Ok(mut rows) => {
                        while let Ok(Some(row)) = rows.next() {
                            let vals: Vec<String> = (0..ncols)
                                .map(|i| {
                                    row.get::<_, String>(i)
                                        .or_else(|_| row.get::<_, i64>(i).map(|v| v.to_string()))
                                        .or_else(|_| row.get::<_, f64>(i).map(|v| v.to_string()))
                                        .or_else(|_| row.get::<_, bool>(i).map(|v| v.to_string()))
                                        .unwrap_or_else(|_| "NULL".into())
                                })
                                .collect();
                            println!("{}", vals.join("\t"));
                        }
                    }
                    Err(e) => println!("Error: {}", e),
                }
            }
            Err(e) => println!("Error: {}", e),
        }
    }
}
