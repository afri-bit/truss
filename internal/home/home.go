// Package home locates the TRUSS home directory, where the daemon keeps its
// state: the registry, run history, the schedule clock and its own endpoint.
package home

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnvVar names the environment variable that overrides the home directory.
const EnvVar = "TRUSS_HOME"

// Dir is a TRUSS home directory.
type Dir string

// Resolve returns the home directory for the given environment ("KEY=value"
// entries): $TRUSS_HOME when set, otherwise ".truss" in the user's home.
func Resolve(env []string) (Dir, error) {
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, EnvVar+"="); ok && v != "" {
			abs, err := filepath.Abs(v)
			if err != nil {
				return "", fmt.Errorf("resolve %s: %w", EnvVar, err)
			}
			return Dir(abs), nil
		}
	}
	user, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find the home directory (set %s to choose one): %w", EnvVar, err)
	}
	return Dir(filepath.Join(user, ".truss")), nil
}

// Path joins elem onto the home directory.
func (d Dir) Path(elem ...string) string {
	return filepath.Join(append([]string{string(d)}, elem...)...)
}

// Endpoint is the file a running daemon writes so clients can find it.
func (d Dir) Endpoint() string { return d.Path("daemon.json") }

// Registry is the file that lists registered pipelines.
func (d Dir) Registry() string { return d.Path("registry.json") }

// Clock is the file that remembers when the scheduler last looked at the time.
func (d Dir) Clock() string { return d.Path("clock.json") }

// Secrets is the file that holds stored secrets (see internal/secrets).
func (d Dir) Secrets() string { return d.Path("secrets.json") }

// Runs is the directory that holds one folder per run.
func (d Dir) Runs() string { return d.Path("runs") }

// DaemonLog is the daemon's own log file.
func (d Dir) DaemonLog() string { return d.Path("daemon.log") }

// Ensure creates the home directory, private to the user.
func (d Dir) Ensure() error {
	if err := os.MkdirAll(d.Runs(), 0o700); err != nil {
		return fmt.Errorf("prepare TRUSS home: %w", err)
	}
	return nil
}

// WriteJSON stores v at path, replacing the file in one step so a reader
// never sees half a file. The file is private to the user.
func WriteJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", filepath.Base(path), err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(path) // Windows cannot always rename over a file
		if err = os.Rename(tmp, path); err != nil {
			return fmt.Errorf("replace %s: %w", filepath.Base(path), err)
		}
	}
	return nil
}

// ReadJSON loads path into v. A missing file is reported by os.IsNotExist.
func ReadJSON(path string, v any) error {
	data, err := os.ReadFile(path) // #nosec G304 -- paths are built from the home directory
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("decode %s: %w", filepath.Base(path), err)
	}
	return nil
}
