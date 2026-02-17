require 'duckdb'

path = ARGV[0] || ':memory:'
db = DuckDB::Database.open(path)
conn = db.connect
interactive = $stdin.tty?

def print_result(result)
  if result.respond_to?(:columns)
    puts result.columns.map(&:name).join("\t")
  end
  result.each do |row|
    puts row.to_a.map { |v| v.nil? ? 'NULL' : v.to_s }.join("\t")
  end
end

def run_sql(conn, sql)
  result = conn.query(sql)
  print_result(result)
rescue => e
  puts "Error: #{e.message}"
end

if interactive
  result = conn.query("SELECT version()")
  version = result.first.first
  puts "puddle DuckDB #{version} (Ruby)"
  puts 'Enter ".quit" to exit.'
end

buf = []
loop do
  if interactive
    prompt = buf.empty? ? "Ruby:D " : "Ruby:.. "
    print prompt
  end

  line = $stdin.gets
  if line.nil?
    puts if interactive
    break
  end

  trimmed = line.strip
  break if buf.empty? && ['.quit', '.exit'].include?(trimmed)

  buf << line.chomp
  sql = buf.join("\n").strip
  next unless sql.end_with?(';')
  buf.clear

  run_sql(conn, sql)
end

# Execute any remaining buffered SQL on EOF.
unless buf.empty?
  sql = buf.join("\n").strip
  run_sql(conn, sql) unless sql.empty?
end
