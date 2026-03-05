package lang

import (
	"fmt"
	"sort"
)

// Language describes a DuckDB language binding.
type Language struct {
	Name           string // human-readable name
	Dir            string // directory under dockerfiles/
	DefaultDuckDB  string // default DuckDB version
	DefaultRuntime string // default language runtime version
	VersionRange   string // human-readable supported DuckDB version range
}

// Registry holds all known languages.
var Registry = map[string]Language{
	"go": {
		Name:           "Go",
		Dir:            "go",
		DefaultDuckDB:  "1.4.4",
		DefaultRuntime: "1.24",
		VersionRange:   "1.2.x - 1.4.4 (via SDK mapping)",
	},
	"python": {
		Name:           "Python",
		Dir:            "python",
		DefaultDuckDB:  "1.4.4",
		DefaultRuntime: "3.12",
		VersionRange:   "1.2.0 - 1.4.4 (any PyPI release)",
	},
	"java": {
		Name:           "Java",
		Dir:            "java",
		DefaultDuckDB:  "1.4.4",
		DefaultRuntime: "21",
		VersionRange:   "1.2.0 - 1.4.4 (JDBC from Maven Central)",
	},
	"node": {
		Name:           "Node.js",
		Dir:            "node",
		DefaultDuckDB:  "1.4.4",
		DefaultRuntime: "20",
		VersionRange:   "1.1.0+ (@duckdb/node-api)",
	},
	"rust": {
		Name:           "Rust",
		Dir:            "rust",
		DefaultDuckDB:  "1.4.4",
		DefaultRuntime: "1",
		VersionRange:   "1.0.0 - 1.4.4 (duckdb crate, bundled)",
	},
	"php": {
		Name:           "PHP",
		Dir:            "php",
		DefaultDuckDB:  "1.4.4",
		DefaultRuntime: "8.3",
		VersionRange:   "1.2.0 - 1.4.4 (satur.io/duckdb)",
	},
	"ruby": {
		Name:           "Ruby",
		Dir:            "ruby",
		DefaultDuckDB:  "1.4.4",
		DefaultRuntime: "3",
		VersionRange:   "1.2.0 - 1.4.4 (libduckdb + gem)",
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
