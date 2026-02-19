package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config holds the puddle configuration for a session or global default.
type Config struct {
	Lang           string `json:"lang,omitempty"`
	DuckDBVersion  string `json:"duckdb_version,omitempty"`
	RuntimeVersion string `json:"runtime_version,omitempty"`
	LibVersion     string `json:"lib_version,omitempty"`
}

// Dir returns the puddle config directory (~/.config/puddle).
func Dir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "puddle")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "puddle")
}

// SessionsDir returns the sessions directory (~/.config/puddle/sessions).
func SessionsDir() string {
	return filepath.Join(Dir(), "sessions")
}

// GlobalPath returns the path to the global config file.
func GlobalPath() string {
	return filepath.Join(Dir(), "config.json")
}

// SessionPath returns the path to a session config file.
func SessionPath(id string) string {
	return filepath.Join(SessionsDir(), id+".json")
}

// LoadGlobal reads the global config. Returns an empty Config if not found.
func LoadGlobal() Config {
	return loadFile(GlobalPath())
}

// SaveGlobal writes the global config.
func SaveGlobal(c Config) error {
	return saveFile(GlobalPath(), c)
}

// ClearGlobal removes the global config file.
func ClearGlobal() error {
	err := os.Remove(GlobalPath())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// LoadSession reads the active session config from PUDDLE_SESSION env var.
// Returns an empty Config if no session is active.
func LoadSession() Config {
	id := os.Getenv("PUDDLE_SESSION")
	if id == "" {
		return Config{}
	}
	return loadFile(SessionPath(id))
}

// ActiveSessionID returns the current session ID from the environment, or "".
func ActiveSessionID() string {
	return os.Getenv("PUDDLE_SESSION")
}

// SaveSession writes a session config file and returns the session ID.
// If name is empty, a random ID is generated.
func SaveSession(name string, c Config) (string, error) {
	id := name
	if id == "" {
		id = randomID()
	}
	return id, saveFile(SessionPath(id), c)
}

// RemoveSession deletes a session config file.
func RemoveSession(id string) error {
	err := os.Remove(SessionPath(id))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// IsNamedSession reports whether a session ID looks user-chosen (not random hex).
func IsNamedSession(id string) bool {
	// Random IDs are 12 hex chars. Named sessions are anything else.
	if len(id) != 12 {
		return true
	}
	_, err := hex.DecodeString(id)
	return err != nil
}

// ListSessions returns all session IDs found in the sessions directory.
func ListSessions() ([]string, error) {
	entries, err := os.ReadDir(SessionsDir())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		ids = append(ids, strings.TrimSuffix(e.Name(), ".json"))
	}
	return ids, nil
}

// CleanStaleSessions removes unnamed session files that no longer have a
// running shell (best-effort: we just remove all unnamed sessions).
func CleanStaleSessions() (int, error) {
	ids, err := ListSessions()
	if err != nil {
		return 0, err
	}
	var removed int
	for _, id := range ids {
		if IsNamedSession(id) {
			continue
		}
		if err := RemoveSession(id); err == nil {
			removed++
		}
	}
	return removed, nil
}

func loadFile(path string) Config {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}
	}
	return c
}

func saveFile(path string, c Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

func randomID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return hex.EncodeToString(b)
}
