// Package registry remembers which pipeline definitions the daemon knows.
//
// A registration is a name and the absolute path of a definition file. The
// file stays where it is and stays the source of truth: the registry never
// copies its content.
package registry

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/afri-bit/truss/internal/home"
)

// ErrNotFound means no pipeline is registered under the given name.
var ErrNotFound = errors.New("pipeline is not registered")

// ErrExists means the name is already taken by another file.
var ErrExists = errors.New("a pipeline with this name is already registered")

var validName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// Entry is one registered pipeline.
type Entry struct {
	Name string `json:"name"`
	// Path is the absolute path of the definition file.
	Path string `json:"path"`
	// Paused stops the schedules of the pipeline. Manual runs still work.
	Paused       bool      `json:"paused"`
	RegisteredAt time.Time `json:"registered_at"`
	// Chosen means the name was picked at registration (--name), so the name
	// in the file never replaces it on screen. A registration without it
	// follows the file's name.
	Chosen bool `json:"chosen,omitempty"`
}

// Registry is the set of registered pipelines, saved to one JSON file.
// It is safe for concurrent use.
type Registry struct {
	mu      sync.Mutex
	path    string
	entries map[string]Entry
}

type file struct {
	Pipelines []Entry `json:"pipelines"`
}

// Open loads the registry at path. A missing file is an empty registry.
func Open(path string) (*Registry, error) {
	r := &Registry{path: path, entries: map[string]Entry{}}
	var f file
	if err := home.ReadJSON(path, &f); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("load registry: %w", err)
	}
	for _, e := range f.Pipelines {
		r.entries[e.Name] = e
	}
	return r, nil
}

// CheckName reports whether name can be used as a registration name.
func CheckName(name string) error {
	if !validName.MatchString(name) {
		return fmt.Errorf("pipeline name %q must start with a letter or digit and use only letters, digits, '.', '_' and '-'", name)
	}
	return nil
}

// Add registers path under name, a name taken from the file. Registering the
// same name and path again is not an error; a different path under a taken
// name is ErrExists.
func (r *Registry) Add(name, path string, now time.Time) (Entry, error) {
	return r.add(name, path, now, false)
}

// AddChosen is Add for a name the user picked: the entry is marked Chosen.
func (r *Registry) AddChosen(name, path string, now time.Time) (Entry, error) {
	return r.add(name, path, now, true)
}

func (r *Registry) add(name, path string, now time.Time, chosen bool) (Entry, error) {
	if err := CheckName(name); err != nil {
		return Entry{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if old, ok := r.entries[name]; ok {
		if old.Path == path {
			return old, nil
		}
		return Entry{}, fmt.Errorf("%w: %q is %s", ErrExists, name, old.Path)
	}
	if !chosen {
		// A file registered again without a chosen name is the registration it
		// already has, whatever name that carries now. Only --name makes
		// another registration of the same file.
		for _, old := range r.sorted() {
			if old.Path == path {
				return old, nil
			}
		}
	}
	e := Entry{Name: name, Path: path, RegisteredAt: now, Chosen: chosen}
	r.entries[name] = e
	if err := r.save(); err != nil {
		delete(r.entries, name)
		return Entry{}, err
	}
	return e, nil
}

// Remove forgets the pipeline registered under name.
func (r *Registry) Remove(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	old, ok := r.entries[name]
	if !ok {
		return fmt.Errorf("%w: %q", ErrNotFound, name)
	}
	delete(r.entries, name)
	if err := r.save(); err != nil {
		r.entries[name] = old
		return err
	}
	return nil
}

// SetPaused pauses or resumes the schedules of a pipeline.
func (r *Registry) SetPaused(name string, paused bool) (Entry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	old, ok := r.entries[name]
	if !ok {
		return Entry{}, fmt.Errorf("%w: %q", ErrNotFound, name)
	}
	e := old
	e.Paused = paused
	r.entries[name] = e
	if err := r.save(); err != nil {
		r.entries[name] = old
		return Entry{}, err
	}
	return e, nil
}

// SetChosen marks the name of a registration as chosen by the user, so the
// name in its file never replaces it on screen.
func (r *Registry) SetChosen(name string) (Entry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	old, ok := r.entries[name]
	if !ok {
		return Entry{}, fmt.Errorf("%w: %q", ErrNotFound, name)
	}
	e := old
	e.Chosen = true
	r.entries[name] = e
	if err := r.save(); err != nil {
		r.entries[name] = old
		return Entry{}, err
	}
	return e, nil
}

// Get returns the pipeline registered under name.
func (r *Registry) Get(name string) (Entry, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.entries[name]
	return e, ok
}

// List returns every registered pipeline, sorted by name.
func (r *Registry) List() []Entry {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.sorted()
}

func (r *Registry) sorted() []Entry {
	list := make([]Entry, 0, len(r.entries))
	for _, e := range r.entries {
		list = append(list, e)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return list
}

func (r *Registry) save() error {
	return home.WriteJSON(r.path, file{Pipelines: r.sorted()})
}
