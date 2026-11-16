package daemon

import (
	"fmt"

	"github.com/afri-bit/truss/internal/api"
	"github.com/afri-bit/truss/internal/registry"
)

// A registration keeps the name it was registered under (its registered
// name): history, schedules and the data and cache folders hang on it, so it
// never changes. What the daemon calls the pipeline, and what clients show,
// follows pipeline.name in the file (spec 019). Every command accepts either.

// shownNames maps each registered name to the name to show for it. The caller
// holds d.mu. A file's own name is used only when it is a valid registration
// name that no other registration holds or claims; otherwise the registered
// name stays, so one name never points at two pipelines. A name chosen at
// registration is never replaced.
func (d *Daemon) shownNames(entries []registry.Entry) map[string]string {
	claims := make(map[string]int, len(entries))
	want := make(map[string]string, len(entries))
	for _, e := range entries {
		claims[e.Name]++
		cur := d.load(e, d.opts.Now(), false)
		if !e.Chosen && cur.err == nil && cur.def.Name != e.Name && registry.CheckName(cur.def.Name) == nil {
			want[e.Name] = cur.def.Name
			claims[cur.def.Name]++
		}
	}
	shown := make(map[string]string, len(entries))
	for _, e := range entries {
		shown[e.Name] = e.Name
		if n, ok := want[e.Name]; ok && claims[n] == 1 {
			shown[e.Name] = n
		}
	}
	return shown
}

// names returns shownNames for every registration.
func (d *Daemon) names() map[string]string {
	entries := d.registry.List()
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.shownNames(entries)
}

// lookup finds a registration by its registered name or by the name shown for
// it. A registered name always wins: no other registration shows it.
func (d *Daemon) lookup(name string) (registry.Entry, bool) {
	if e, ok := d.registry.Get(name); ok {
		return e, true
	}
	shown := d.names()
	for _, e := range d.registry.List() {
		if shown[e.Name] == name {
			return e, true
		}
	}
	return registry.Entry{}, false
}

// find is lookup that reports an unknown name as registry.ErrNotFound.
func (d *Daemon) find(name string) (registry.Entry, error) {
	e, ok := d.lookup(name)
	if !ok {
		return e, fmt.Errorf("%w: %q", registry.ErrNotFound, name)
	}
	return e, nil
}

// showRun gives a run the name shown for its pipeline. A run's record keeps
// the registered name it ran under.
func showRun(shown map[string]string, run api.Run) api.Run {
	if n, ok := shown[run.Pipeline]; ok {
		run.Pipeline = n
	}
	return run
}

// historyNames are the pipeline names whose runs make up the history of name:
// the registered name and the shown name of its registration, or just name
// when no registration answers to it (a flow that was unregistered).
func (d *Daemon) historyNames(name string) []string {
	e, ok := d.lookup(name)
	if !ok {
		return []string{name}
	}
	if shown := d.names()[e.Name]; shown != e.Name {
		return []string{e.Name, shown}
	}
	return []string{e.Name}
}
