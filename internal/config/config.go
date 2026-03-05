package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds the puddle configuration.
type Config struct {
	Lang           string `json:"lang,omitempty"`
	DuckDBVersion  string `json:"duckdb_version,omitempty"`
	RuntimeVersion string `json:"runtime_version,omitempty"`
}

// Dir returns the puddle config directory (~/.config/puddle).
func Dir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "puddle")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "puddle")
}

// GlobalPath returns the path to the global config file.
func GlobalPath() string {
	return filepath.Join(Dir(), "config.json")
}

// LoadGlobal reads the global config. Returns an empty Config if not found.
func LoadGlobal() Config {
	data, err := os.ReadFile(GlobalPath())
	if err != nil {
		return Config{}
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}
	}
	return c
}

// SaveGlobal writes the global config.
func SaveGlobal(c Config) error {
	path := GlobalPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

// ClearGlobal removes the global config file.
func ClearGlobal() error {
	err := os.Remove(GlobalPath())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
