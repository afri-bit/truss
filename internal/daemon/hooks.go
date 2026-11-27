package daemon

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/afri-bit/truss/internal/api"
	"github.com/afri-bit/truss/internal/pipeline"
)

// Local hooks (spec 029): on.file starts a flow when files change, on.run
// when another flow's run ends. Both act only while the daemon runs.

// maxWatchedFiles bounds what one flow's on.file may watch, so a pattern
// over a huge tree cannot make every tick slow.
const maxWatchedFiles = 10000

// maxChain bounds how many on.run hooks may follow one another, so flows
// that start each other cannot run for ever.
const maxChain = 10

type fileStamp struct {
	mod  time.Time
	size int64
}

// fileWatch is the state of one flow's on.file.
type fileWatch struct {
	hook    pipeline.FileHook
	root    string
	snap    map[string]fileStamp
	changed map[string]bool
	last    time.Time // when the newest change was seen
	tooMany bool      // the last scan hit maxWatchedFiles; logged once
}

// finishedRun is a run that ended, for on.run.
type finishedRun struct {
	run   api.Run
	chain int
}

// checkFileHooks scans the watched paths of every registered, unpaused flow
// with on.file, and starts a run once changes have stopped for the debounce.
// The first scan of a flow only takes a baseline: changes made while the
// daemon was not running start nothing.
func (d *Daemon) checkFileHooks(now time.Time) {
	type target struct {
		name string
		hook pipeline.FileHook
		root string
	}
	var targets []target
	d.mu.Lock()
	for name, def := range d.defs {
		if def.err != nil || def.def.FileHook == nil {
			continue
		}
		e, ok := d.registry.Get(name)
		if !ok || e.Paused {
			continue
		}
		targets = append(targets, target{name, *def.def.FileHook, workingDir(e.Path)})
	}
	d.mu.Unlock()

	seen := map[string]bool{}
	for _, t := range targets {
		seen[t.name] = true
		w := d.watches[t.name]
		if w == nil || w.root != t.root || !sameHook(w.hook, t.hook) {
			// A new or edited hook starts from a fresh baseline.
			w = &fileWatch{hook: t.hook, root: t.root, changed: map[string]bool{}}
			d.watches[t.name] = w
		}
		snap, full := scan(t.root, t.hook)
		if full && !w.tooMany {
			d.logf("flow %s: on.file watches more than %d files; only the first are watched", t.name, maxWatchedFiles)
		}
		w.tooMany = full
		if w.snap != nil {
			for p, st := range snap {
				if old, ok := w.snap[p]; !ok || !old.mod.Equal(st.mod) || old.size != st.size {
					w.changed[p], w.last = true, now
				}
			}
			for p := range w.snap {
				if _, ok := snap[p]; !ok {
					w.changed[p], w.last = true, now
				}
			}
		}
		w.snap = snap
		if len(w.changed) == 0 || now.Sub(w.last) < t.hook.Debounce {
			continue
		}
		changed := make([]string, 0, len(w.changed))
		for p := range w.changed {
			changed = append(changed, p)
		}
		sort.Strings(changed)
		w.changed = map[string]bool{}
		if _, err := d.submitHook(t.name, api.TriggerFile, changed, 0); err != nil {
			d.logf("flow %s: files changed but the run could not start: %v", t.name, err)
		} else {
			d.logf("flow %s: %d file(s) changed -> run", t.name, len(changed))
		}
	}
	for name := range d.watches {
		if !seen[name] {
			delete(d.watches, name)
		}
	}
}

func sameHook(a, b pipeline.FileHook) bool {
	return strings.Join(a.Paths, "\x00") == strings.Join(b.Paths, "\x00") &&
		strings.Join(a.Ignore, "\x00") == strings.Join(b.Ignore, "\x00")
}

// scan lists the files the hook watches, by path relative to root (with
// slashes; an absolute pattern gives an absolute path). It follows no link,
// never looks in .truss/state/ or .git/, and stops at maxWatchedFiles.
func scan(root string, hook pipeline.FileHook) (map[string]fileStamp, bool) {
	out := map[string]fileStamp{}
	full := false
	for _, pattern := range hook.Paths {
		base := pipeline.PatternBase(pattern)
		start := filepath.FromSlash(base)
		if !filepath.IsAbs(start) {
			start = filepath.Join(root, start)
		}
		_ = filepath.WalkDir(start, func(path string, de fs.DirEntry, err error) error {
			if err != nil {
				return nil // a folder that went away or cannot be read
			}
			rel := path
			if r, err := filepath.Rel(root, path); err == nil && !strings.HasPrefix(r, "..") && !filepath.IsAbs(base) {
				rel = r
			}
			rel = filepath.ToSlash(rel)
			if de.IsDir() {
				if rel == ".git" || rel == ".truss/state" || strings.HasSuffix(rel, "/.git") {
					return filepath.SkipDir
				}
				return nil
			}
			if !de.Type().IsRegular() || !pipeline.MatchPattern(pattern, rel) || ignored(hook.Ignore, rel) {
				return nil
			}
			if len(out) >= maxWatchedFiles {
				full = true
				return fs.SkipAll
			}
			info, err := de.Info()
			if err != nil {
				return nil
			}
			out[rel] = fileStamp{info.ModTime(), info.Size()}
			return nil
		})
	}
	return out, full
}

func ignored(patterns []string, rel string) bool {
	for _, p := range patterns {
		if pipeline.MatchPattern(p, rel) {
			return true
		}
	}
	return false
}

// runFinished starts the flows whose on.run matches a run that ended.
func (d *Daemon) runFinished(f finishedRun) {
	type match struct{ name string }
	var starts []match
	d.mu.Lock()
	for name, def := range d.defs {
		if def.err != nil {
			continue
		}
		for _, h := range def.def.RunHooks {
			if h.Matches(f.run.Pipeline, f.run.Status) || (f.run.RegisteredPipeline != "" && h.Matches(f.run.RegisteredPipeline, f.run.Status)) {
				starts = append(starts, match{name})
				break
			}
		}
	}
	d.mu.Unlock()
	for _, m := range starts {
		if e, ok := d.registry.Get(m.name); !ok || e.Paused {
			continue
		}
		if f.chain+1 > maxChain {
			d.logf("flow %s: on.run not started: %d flows already started each other in a row (a loop?)", m.name, maxChain)
			continue
		}
		if _, err := d.submitHook(m.name, api.TriggerRun, nil, f.chain+1); err != nil {
			d.logf("flow %s: run of %s ended (%s) but the run could not start: %v", m.name, f.run.Pipeline, f.run.Status, err)
		} else {
			d.logf("flow %s: run of %s ended (%s) -> run", m.name, f.run.Pipeline, f.run.Status)
		}
	}
}
