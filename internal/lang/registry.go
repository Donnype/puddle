package lang

import (
	"fmt"
	"sort"
)

// Language describes a DuckDB language binding.
type Language struct {
	Name              string // human-readable name
	Dir               string // directory under dockerfiles/
	DuckDBVersionArg  string // Docker build arg for DuckDB version (empty = not configurable)
	LibVersionArg     string // Docker build arg for library version (empty = N/A)
	RuntimeVersionArg string // Docker build arg for language runtime version
	DefaultDuckDB     string // default DuckDB version
	DefaultLib        string // default library version
	DefaultRuntime    string // default language runtime version
	VersionRange      string // human-readable supported DuckDB version range
	NativeBin         string // default binary for native (non-Docker) mode
}

// Registry holds all known languages.
var Registry = map[string]Language{
	"go": {
		Name:              "Go",
		Dir:               "go",
		DuckDBVersionArg:  "DUCKDB_VERSION",
		RuntimeVersionArg: "GO_VERSION",
		DefaultDuckDB:     "1.4.4",
		DefaultRuntime:    "1.24",
		VersionRange:      "1.2.x - 1.4.4 (via SDK mapping)",
		NativeBin:         "go",
	},
	"python": {
		Name:              "Python",
		Dir:               "python",
		DuckDBVersionArg:  "DUCKDB_VERSION",
		RuntimeVersionArg: "PYTHON_VERSION",
		DefaultDuckDB:     "1.4.4",
		DefaultRuntime:    "3.12",
		VersionRange:      "1.2.0 - 1.4.4 (any PyPI release)",
		NativeBin:         "python3",
	},
	"java": {
		Name:              "Java",
		Dir:               "java",
		DuckDBVersionArg:  "DUCKDB_VERSION",
		RuntimeVersionArg: "JAVA_VERSION",
		DefaultDuckDB:     "1.4.4",
		DefaultRuntime:    "21",
		VersionRange:      "1.2.0 - 1.4.4 (JDBC from Maven Central)",
		NativeBin:         "java",
	},
	"node": {
		Name:              "Node.js",
		Dir:               "node",
		DuckDBVersionArg:  "DUCKDB_VERSION",
		RuntimeVersionArg: "NODE_VERSION",
		DefaultDuckDB:     "1.4.4",
		DefaultRuntime:    "20",
		VersionRange:      "1.1.0+ (@duckdb/node-api)",
		NativeBin:         "node",
	},
	"rust": {
		Name:              "Rust",
		Dir:               "rust",
		DuckDBVersionArg:  "DUCKDB_VERSION",
		RuntimeVersionArg: "RUST_VERSION",
		DefaultDuckDB:     "1.4.4",
		DefaultRuntime:    "1",
		VersionRange:      "1.0.0 - 1.4.4 (duckdb crate, bundled)",
		NativeBin:         "cargo",
	},
	"php": {
		Name:              "PHP",
		Dir:               "php",
		DuckDBVersionArg:  "DUCKDB_VERSION",
		LibVersionArg:     "DUCKDB_PHP_VERSION",
		RuntimeVersionArg: "PHP_VERSION",
		DefaultDuckDB:     "1.4.4",
		DefaultLib:        "2.0.3",
		DefaultRuntime:    "8.3",
		VersionRange:      "1.2.0 - 1.4.4 (satur.io/duckdb)",
		NativeBin:         "php",
	},
	"ruby": {
		Name:              "Ruby",
		Dir:               "ruby",
		DuckDBVersionArg:  "DUCKDB_VERSION",
		RuntimeVersionArg: "RUBY_VERSION",
		DefaultDuckDB:     "1.4.4",
		DefaultRuntime:    "3",
		VersionRange:      "1.2.0 - 1.4.4 (libduckdb + gem)",
		NativeBin:         "ruby",
	},
}

// Get returns a language by key, or an error if not found.
func Get(name string) (Language, error) {
	l, ok := Registry[name]
	if !ok {
		return Language{}, fmt.Errorf("unknown language %q (use 'puddle list' to see available languages)", name)
	}
	return l, nil
}

// Names returns sorted language keys.
func Names() []string {
	names := make([]string, 0, len(Registry))
	for k := range Registry {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}
