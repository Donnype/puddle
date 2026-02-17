import { DuckDBInstance } from '@duckdb/node-api';
import { createInterface } from 'readline/promises';
import { stdin, stdout } from 'process';

const instance = await DuckDBInstance.create();
const conn = await instance.connect();
const interactive = stdin.isTTY === true;

if (interactive) {
    const vr = await conn.runAndReadAll("SELECT version()");
    const version = vr.getRows()[0][0];
    console.log(`puddle DuckDB ${version} (Node.js)`);
    console.log('Enter ".quit" to exit.');
}

const rl = createInterface({ input: stdin, output: interactive ? stdout : undefined });

let buf = [];
try {
    while (true) {
        const prompt = interactive ? (buf.length === 0 ? 'Node:D ' : 'Node:.. ') : '';
        let line;
        try {
            line = await rl.question(prompt);
        } catch {
            if (interactive) console.log();
            break;
        }

        const trimmed = line.trim();
        if (buf.length === 0 && (trimmed === '.quit' || trimmed === '.exit')) break;

        buf.push(line);
        const sql = buf.join('\n').trim();
        if (!sql.endsWith(';')) continue;
        buf = [];

        try {
            const reader = await conn.runAndReadAll(sql);
            const cols = reader.columnNames();
            const rows = reader.getRows();
            if (cols.length > 0 && rows.length > 0) {
                console.log(cols.join('\t'));
                for (const row of rows) {
                    console.log(row.map(v => v === null || v === undefined ? 'NULL' : String(v)).join('\t'));
                }
            }
        } catch (e) {
            console.log(`Error: ${e.message}`);
        }
    }

    // Execute any remaining buffered SQL on EOF.
    if (buf.length > 0) {
        const sql = buf.join('\n').trim();
        if (sql) {
            try {
                const reader = await conn.runAndReadAll(sql);
                const cols = reader.columnNames();
                const rows = reader.getRows();
                if (cols.length > 0 && rows.length > 0) {
                    console.log(cols.join('\t'));
                    for (const row of rows) {
                        console.log(row.map(v => v === null || v === undefined ? 'NULL' : String(v)).join('\t'));
                    }
                }
            } catch (e) {
                console.log(`Error: ${e.message}`);
            }
        }
    }
} finally {
    rl.close();
}
