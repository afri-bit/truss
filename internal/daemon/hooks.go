package daemon

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/afri-bit/truss/internal/api"
	"github.com/afri-bit/truss/internal/pipeline"
)

// Local hooks (spec 029): on.file starts a flow when files change, on.run
// when another flow's run ends. Both act only while the daemon runs.

// maxWatchedFiles bounds what one flow's on.file may watch, and
// maxVisited what one scan may look at (folders and files that do not match
// included), so a pattern over a huge tree cannot make every tick slow.
const (
	maxWatchedFiles = 10000
	maxVisited      = 50000
)

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
	tooMany bool      // the last scan hit a limit; logged once
	noted   map[string]bool
}

// scanResult is what one scan found.
type scanResult struct {
	files map[string]fileStamp
	// limit is set when the scan stopped at maxWatchedFiles or maxVisited.
	limit string
	// skipped are names left out: links at a pattern's base, and names
	// with control characters, which a list of lines cannot hold safely.
	skipped []string
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
		res := scan(t.root, t.hook)
		snap := res.files
		if res.limit != "" && !w.tooMany {
			d.logf("flow %s: on.file %s; narrow its paths", t.name, res.limit)
		}
		w.tooMany = res.limit != ""
		if w.noted == nil {
			w.noted = map[string]bool{}
		}
		for _, why := range res.skipped {
			if !w.noted[why] {
				w.noted[why] = true
				d.logf("flow %s: on.file %s", t.name, why)
			}
		}
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
		if d.hookRunWaiting(t.name) {
			continue // the queued run will see the files as they are; keep the changes for later
		}
		w.changed = map[string]bool{}
		if _, err := d.submitHook(t.name, api.TriggerFile, changed, 0); err != nil {
			d.logf("flow %s: files changed but the run could not start: %v", t.name, err)
			for _, p := range changed {
				w.changed[p] = true // try again at the next quiet moment
			}
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
// never looks in .truss/state/ or .git/, and stops at maxWatchedFiles files
// or maxVisited entries across all patterns.
func scan(root string, hook pipeline.FileHook) scanResult {
	res := scanResult{files: map[string]fileStamp{}}
	visited := 0
	for _, pattern := range hook.Paths {
		if res.limit != "" {
			break
		}
		base := pipeline.PatternBase(pattern)
		start := filepath.FromSlash(base)
		absolute := filepath.IsAbs(start)
		if !absolute {
			start = filepath.Join(root, start)
		}
		if info, err := os.Lstat(start); err == nil && info.Mode()&fs.ModeSymlink != 0 {
			res.skipped = append(res.skipped, "does not follow the link "+filepath.ToSlash(base)+"; name the folder it points to")
			continue
		}
		_ = filepath.WalkDir(start, func(path string, de fs.DirEntry, err error) error {
			if visited++; visited > maxVisited {
				res.limit = fmt.Sprintf("looked at more than %d files and folders and stopped", maxVisited)
				return fs.SkipAll
			}
			if err != nil {
				return nil // a folder that went away or cannot be read
			}
			rel := filepath.ToSlash(path)
			if !absolute {
				if r, err := filepath.Rel(root, path); err == nil {
					rel = filepath.ToSlash(r)
				}
			}
			if de.IsDir() {
				if rel == ".git" || rel == ".truss/state" || strings.HasSuffix(rel, "/.git") || ignored(hook.Ignore, rel) {
					return filepath.SkipDir // an ignored folder is not even looked into
				}
				return nil
			}
			if !de.Type().IsRegular() || !pipeline.MatchPattern(pattern, rel) || ignored(hook.Ignore, rel) {
				return nil
			}
			if strings.ContainsFunc(rel, unicode.IsControl) {
				res.skipped = append(res.skipped, "leaves out a file whose name has a control character (such as a line break)")
				return nil
			}
			if len(res.files) >= maxWatchedFiles {
				res.limit = fmt.Sprintf("matches more than %d files; only the first are watched", maxWatchedFiles)
				return fs.SkipAll
			}
			info, err := de.Info()
			if err != nil {
				return nil
			}
			res.files[rel] = fileStamp{info.ModTime(), info.Size()}
			return nil
		})
	}
	return res
}

// hookRunWaiting reports whether a run of the flow that a hook started is
// still queued or running: hooks give a flow at most one such run at a time,
// so flows that start each other cannot multiply.
func (d *Daemon) hookRunWaiting(name string) bool {
	for _, r := range d.runs.List(name, 0) {
		if r.Active() && (r.Trigger == api.TriggerFile || r.Trigger == api.TriggerRun) {
			return true
		}
	}
	return false
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
	// A run that never started (cancelled while queued, skipped, or a file
	// that did not load) starts no hook.
	if f.run.StartedAt == nil {
		return
	}
	d.mu.Lock()
	shown := f.run.Pipeline
	if def, ok := d.defs[f.run.Pipeline]; ok && def.def.Name != "" {
		shown = def.def.Name
	}
	for name, def := range d.defs {
		if def.err != nil {
			continue
		}
		for _, h := range def.def.RunHooks {
			if h.Matches(f.run.Pipeline, f.run.Status) || h.Matches(shown, f.run.Status) {
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
		if d.hookRunWaiting(m.name) {
			continue // one waiting hook run per flow, so flows cannot fan out without end
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
