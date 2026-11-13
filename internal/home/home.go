// Package home locates the TRUSS home directory, where the daemon keeps its
// state: the registry, run history, the schedule clock and its own endpoint.
package home

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
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

// Pipelines is the directory that holds one data folder per pipeline name.
func (d Dir) Pipelines() string { return d.Path("pipelines") }

// PipelineData is the data folder of the registered pipeline called name. The
// caller must have checked that name is a valid registry name.
func (d Dir) PipelineData(name string) string { return d.Path("pipelines", name) }

// PipelineCache is the cache folder inside PipelineData.
func (d Dir) PipelineCache(name string) string { return d.Path("pipelines", name, "cache") }

// ClearPipelineCache removes the cache folder of the pipeline called name and
// reports whether there was one. The rest of the data folder stays.
func (d Dir) ClearPipelineCache(name string) (existed bool, err error) {
	dir := d.PipelineCache(name)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return false, nil
	}
	if err := os.RemoveAll(dir); err != nil {
		return true, fmt.Errorf("clear the cache of %s: %w", name, err)
	}
	return true, nil
}

// DaemonLog is the daemon's own log file.
func (d Dir) DaemonLog() string { return d.Path("daemon.log") }

// Ensure creates the home directory, private to the user. A directory that
// already exists is tightened to 0700 on systems that honor file modes; one
// that cannot be tightened (for example it belongs to someone else) is an
// error, because the endpoint file with the token lives in it.
func (d Dir) Ensure() error {
	if err := os.MkdirAll(d.Runs(), 0o700); err != nil {
		return fmt.Errorf("prepare TRUSS home: %w", err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(string(d), 0o700); err != nil {
			return fmt.Errorf("make the TRUSS home private (%s): %w", d, err)
		}
	}
	return nil
}

// WriteJSON stores v at path, replacing the file in one step so a reader
// never sees half a file. The file is private to the user: it is written to a
// new, uniquely named temporary file (mode 0600) in the same directory, so a
// leftover or planted file at a predictable name is never reused or followed.
func WriteJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", filepath.Base(path), err)
	}
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	tmp := f.Name()
	_, werr := f.Write(append(data, '\n'))
	if werr == nil {
		werr = f.Sync()
	}
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("write %s: %w", filepath.Base(path), werr)
	}
	if err := rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace %s: %w", filepath.Base(path), err)
	}
	return nil
}

// rename replaces newpath with oldpath. On Windows a reader that holds the
// destination open can make this fail for a moment, so it is retried briefly.
// The destination is never removed first: a failed replace leaves the old file.
func rename(oldpath, newpath string) error {
	var err error
	for attempt := 0; attempt < 50; attempt++ {
		if err = os.Rename(oldpath, newpath); err == nil || runtime.GOOS != "windows" {
			return err
		}
		time.Sleep(20 * time.Millisecond)
	}
	return err
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
