require 'duckdb'

path = ARGV[0] || ':memory:'
db = DuckDB::Database.open(path)
conn = db.connect

result = conn.query("SELECT version()")
version = result.first.first
puts "puddle DuckDB #{version} (Ruby)"
puts 'Enter ".quit" to exit.'

buf = []
loop do
  prompt = buf.empty? ? "Ruby:D " : "Ruby:.. "
  print prompt

  line = $stdin.gets
  if line.nil?
    puts
    break
  end

  trimmed = line.strip
  break if buf.empty? && ['.quit', '.exit'].include?(trimmed)

  buf << line.chomp
  sql = buf.join("\n").strip
  next unless sql.end_with?(';')
  buf.clear

  begin
    result = conn.query(sql)
    if result.respond_to?(:columns)
      puts result.columns.map(&:name).join("\t")
    end
    result.each do |row|
      puts row.to_a.map { |v| v.nil? ? 'NULL' : v.to_s }.join("\t")
    end
  rescue => e
    puts "Error: #{e.message}"
  end
end
