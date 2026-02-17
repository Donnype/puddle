<?php
require_once __DIR__ . '/vendor/autoload.php';

use Saturio\DuckDB\DuckDB;

$path = $argv[1] ?? ':memory:';
$db = DuckDB::create($path);

echo "puddle DuckDB (PHP)\n";
$db->query("SELECT version() AS version")->print();
echo 'Enter ".quit" to exit.' . "\n";

$buf = [];
while (true) {
    $prompt = empty($buf) ? "PHP:D " : "PHP:.. ";
    echo $prompt;

    $line = fgets(STDIN);
    if ($line === false) {
        echo "\n";
        break;
    }

    $trimmed = trim($line);
    if (empty($buf) && in_array($trimmed, ['.quit', '.exit'])) break;

    $buf[] = rtrim($line, "\n");
    $sql = trim(implode("\n", $buf));

    if (!str_ends_with($sql, ';')) continue;
    $buf = [];

    try {
        $result = $db->query($sql);
        if ($result) {
            $result->print();
        }
    } catch (\Throwable $e) {
        echo "Error: " . $e->getMessage() . "\n";
    }
}
