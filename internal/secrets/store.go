package secrets

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
)

// ErrNotFound means the store has no secret with the given name.
var ErrNotFound = errors.New("no such secret")

var validName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// ValidName reports whether name can be used as a secret name: the same
// rule as an environment variable name.
func ValidName(name string) bool { return validName.MatchString(name) }

// Store is the secrets file. The zero value is not usable; use Open.
type Store struct {
	path string
}

// Open returns the store kept in the file at path. The file need not exist.
func Open(path string) *Store { return &Store{path: path} }

// Path is the file that holds the secrets.
func (s *Store) Path() string { return s.path }

type fileFormat struct {
	Version int               `json:"version"`
	Secrets map[string]string `json:"secrets"`
}

// Load returns every secret, name to value. A missing file is an empty store.
// On Linux and macOS a file that group or others can access is refused.
func (s *Store) Load() (map[string]string, error) {
	info, err := os.Stat(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read secrets: %w", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("secrets file %s is accessible to other users (mode %04o); run: chmod 600 %s",
			s.path, info.Mode().Perm(), s.path)
	}
	data, err := os.ReadFile(s.path) // #nosec G304 -- the path is the store file in the TRUSS home
	if err != nil {
		return nil, fmt.Errorf("read secrets: %w", err)
	}
	var f fileFormat
	if err := json.Unmarshal(data, &f); err != nil {
		// The decoder's message can quote file content, so it is not repeated.
		return nil, fmt.Errorf("secrets file %s is not valid: it must be JSON written by truss secret set", s.path)
	}
	if f.Secrets == nil {
		f.Secrets = map[string]string{}
	}
	return f.Secrets, nil
}

// Names returns the stored names, sorted.
func (s *Store) Names() ([]string, error) {
	all, err := s.Load()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(all))
	for n := range all {
		names = append(names, n)
	}
	sort.Strings(names)
	return names, nil
}

// Set stores value under name, replacing any earlier value.
func (s *Store) Set(name, value string) error {
	if !ValidName(name) {
		return fmt.Errorf("%q is not a valid secret name: use letters, digits and '_', not starting with a digit", name)
	}
	if value == "" {
		return errors.New("a secret value must not be empty")
	}
	all, err := s.Load()
	if err != nil {
		return err
	}
	all[name] = value
	return s.save(all)
}

// Remove deletes the secret called name. A missing name is ErrNotFound.
func (s *Store) Remove(name string) error {
	all, err := s.Load()
	if err != nil {
		return err
	}
	if _, ok := all[name]; !ok {
		return fmt.Errorf("%w: %s", ErrNotFound, name)
	}
	delete(all, name)
	return s.save(all)
}

// save replaces the file in one step. The temporary file is created private
// to the user before any value is written to it.
func (s *Store) save(all map[string]string) error {
	data, err := json.MarshalIndent(fileFormat{Version: 1, Secrets: all}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode secrets: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("prepare secrets directory: %w", err)
	}
	// CreateTemp makes a uniquely named file, private to the user on Unix, so
	// concurrent writers never share or inherit a temporary file.
	f, err := os.CreateTemp(filepath.Dir(s.path), "secrets-*.tmp")
	if err != nil {
		return fmt.Errorf("write secrets: %w", err)
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
		return fmt.Errorf("write secrets: %w", werr)
	}
	// Rename replaces the file on every supported OS. The live store is never
	// deleted first: a failed replace leaves the old secrets intact.
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace secrets: %w", err)
	}
	return nil
}
