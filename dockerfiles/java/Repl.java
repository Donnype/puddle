import java.sql.*;
import java.io.*;

public class Repl {
    public static void main(String[] args) throws Exception {
        String url = args.length > 0 ? "jdbc:duckdb:" + args[0] : "jdbc:duckdb:";
        Connection conn = DriverManager.getConnection(url);
        boolean interactive = System.console() != null;

        if (interactive) {
            Statement st = conn.createStatement();
            ResultSet rs = st.executeQuery("SELECT version()");
            rs.next();
            System.out.println("puddle DuckDB " + rs.getString(1) + " (Java)");
            System.out.println("Enter \".quit\" to exit.");
            rs.close();
            st.close();
        }

        BufferedReader reader = new BufferedReader(new InputStreamReader(System.in));
        StringBuilder buf = new StringBuilder();

        while (true) {
            if (interactive) {
                System.out.print(buf.length() == 0 ? "Java:D " : "Java:.. ");
                System.out.flush();
            }

            String line = reader.readLine();
            if (line == null) {
                if (interactive) System.out.println();
                break;
            }

            String trimmed = line.trim();
            if (buf.length() == 0 && (trimmed.equals(".quit") || trimmed.equals(".exit"))) {
                break;
            }

            if (buf.length() > 0) buf.append("\n");
            buf.append(line);

            String sql = buf.toString().trim();
            if (!sql.endsWith(";")) continue;
            buf.setLength(0);

            executeAndPrint(conn, sql);
        }

        // Execute any remaining buffered SQL on EOF.
        if (buf.length() > 0) {
            String sql = buf.toString().trim();
            if (!sql.isEmpty()) {
                executeAndPrint(conn, sql);
            }
        }

        conn.close();
    }

    private static void executeAndPrint(Connection conn, String sql) {
        try {
            Statement stmt = conn.createStatement();
            boolean hasResults = stmt.execute(sql);
            if (hasResults) {
                ResultSet result = stmt.getResultSet();
                ResultSetMetaData meta = result.getMetaData();
                int colCount = meta.getColumnCount();

                StringBuilder header = new StringBuilder();
                for (int i = 1; i <= colCount; i++) {
                    if (i > 1) header.append("\t");
                    header.append(meta.getColumnName(i));
                }
                System.out.println(header);

                while (result.next()) {
                    StringBuilder row = new StringBuilder();
                    for (int i = 1; i <= colCount; i++) {
                        if (i > 1) row.append("\t");
                        Object val = result.getObject(i);
                        row.append(val != null ? val.toString() : "NULL");
                    }
                    System.out.println(row);
                }
                result.close();
            }
            stmt.close();
        } catch (SQLException e) {
            System.out.println("Error: " + e.getMessage());
        }
    }
}
