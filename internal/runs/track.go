package runs

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/afri-bit/truss/internal/api"
)

// ErrInvalid means a request to track a run is malformed.
var ErrInvalid = errors.New("invalid request")

// ErrConflict means a request does not fit the run's current state: it is not
// a tracked run, it already ended, or a log append does not continue the log.
var ErrConflict = errors.New("conflicts with the run's state")

// ErrTooLarge means a request carries more than the API allows.
var ErrTooLarge = errors.New("request too large")

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
}

func conflict(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrConflict, fmt.Sprintf(format, args...))
}

// A tracked run executes outside the daemon, in a caller's shell. The daemon
// only records what the caller reports, so such a run never takes a slot of
// the run limit and never blocks a pipeline's overlap policy.
func tracked(rec *record) bool { return rec.run.Trigger == api.TriggerLocal }

// Track opens a run that executes elsewhere. It starts in state running, with
// every step pending, and has the trigger api.TriggerLocal.
func (m *Manager) Track(name string, steps []api.PlanStep) (api.Run, error) {
	if name == "" || len(name) > api.MaxTrackedName || !utf8.ValidString(name) || hasControl(name) {
		return api.Run{}, invalid("pipeline must be 1 to %d printable characters", api.MaxTrackedName)
	}
	if len(steps) == 0 || len(steps) > api.MaxTrackedSteps {
		return api.Run{}, invalid("a run declares 1 to %d steps", api.MaxTrackedSteps)
	}
	seen := make(map[string]bool, len(steps))
	for _, s := range steps {
		if s.ID == "" || len(s.ID) > api.MaxTrackedName || !utf8.ValidString(s.ID) || hasControl(s.ID) || seen[s.ID] {
			return api.Run{}, invalid("step id %q is empty, too long or repeated", s.ID)
		}
		seen[s.ID] = true
	}
	for _, s := range steps {
		if len(s.Needs) > len(steps) {
			return api.Run{}, invalid("step %q needs more steps than the run has", s.ID)
		}
		for _, n := range s.Needs {
			if !seen[n] {
				return api.Run{}, invalid("step %q needs %q, which the run does not declare", s.ID, n)
			}
		}
	}
	if hasCycle(steps) {
		return api.Run{}, invalid("the steps need each other in a cycle")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return api.Run{}, ErrClosed
	}
	open := 0
	for _, r := range m.records {
		if tracked(r) && r.run.Status == api.RunRunning {
			open++
		}
	}
	if open >= api.MaxOpenTracked {
		return api.Run{}, conflict("%d tracked runs are already running", open)
	}
	rec := m.newRecord(name, api.TriggerLocal, zeroTime)
	for _, s := range steps {
		rec.run.Steps = append(rec.run.Steps, api.Step{ID: s.ID, Needs: s.Needs, Status: api.StepPending})
	}
	now := m.opts.Now()
	rec.run.Status, rec.run.StartedAt = api.RunRunning, &now
	return m.add(rec)
}

// TrackStep records the state of one step of a tracked run.
func (m *Manager) TrackStep(id string, u api.StepUpdate) (api.Run, error) {
	switch u.Status {
	case api.StepRunning, api.StepSucceeded, api.StepFailed, api.StepStopped, api.StepCancelled:
	default:
		return api.Run{}, invalid("step status %q is not one of running, succeeded, failed, stopped, cancelled", u.Status)
	}
	if len(u.Error) > api.MaxReason {
		return api.Run{}, ErrTooLarge
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, err := m.openTracked(id)
	if err != nil {
		return api.Run{}, err
	}
	now := m.opts.Now()
	for i := range rec.run.Steps {
		step := &rec.run.Steps[i]
		if step.ID != u.ID {
			continue
		}
		if step.Status != api.StepPending && step.Status != api.StepRunning {
			return api.Run{}, conflict("step %s already %s", step.ID, step.Status)
		}
		step.Status = u.Status
		if step.StartedAt == nil {
			step.StartedAt = &now
		}
		if u.Status != api.StepRunning {
			step.FinishedAt, step.ExitCode, step.Error = &now, u.ExitCode, plain(u.Error)
		}
		if err := m.save(rec); err != nil {
			return api.Run{}, err
		}
		return rec.run.Copy(), nil
	}
	return api.Run{}, invalid("the run has no step %q", u.ID)
}

// TrackLog appends data to the log of a tracked run. offset must be the end of
// the log so far. A repeated append that the log already holds is accepted and
// changes nothing, so a caller can retry after a lost answer. It returns the
// new end of the log and the run's status.
func (m *Manager) TrackLog(id string, offset int64, data []byte) (int64, string, error) {
	if len(data) > api.MaxLogChunk {
		return 0, "", ErrTooLarge
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, err := m.openTracked(id)
	if err != nil {
		return 0, "", err
	}
	f, err := os.OpenFile(m.logPath(id), os.O_CREATE|os.O_RDWR, 0o600) // #nosec G304 -- the id was looked up in the history
	if err != nil {
		return 0, "", fmt.Errorf("open run log: %w", err)
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return 0, "", fmt.Errorf("read run log: %w", err)
	}
	size := info.Size()
	switch {
	case offset == size:
		if size+int64(len(data)) > api.MaxTrackedLog {
			return 0, "", ErrTooLarge
		}
		if _, err := f.WriteAt(data, size); err != nil {
			return 0, "", fmt.Errorf("write run log: %w", err)
		}
		size += int64(len(data))
	case offset >= 0 && offset+int64(len(data)) <= size:
		// A repeat of what the log holds is accepted: an earlier answer was lost.
		held := make([]byte, len(data))
		if _, err := f.ReadAt(held, offset); err != nil || !bytes.Equal(held, data) {
			return 0, "", conflict("the log already holds different text at offset %d", offset)
		}
	default:
		return 0, "", conflict("the log is %d bytes long; an append must start at its end", size)
	}
	return size, rec.run.Status, nil
}

// TrackFinish closes a tracked run. status is succeeded, failed or cancelled.
// Steps that are still open close with it.
func (m *Manager) TrackFinish(id, status, reason string) (api.Run, error) {
	switch status {
	case api.RunSucceeded, api.RunFailed, api.RunCancelled:
	default:
		return api.Run{}, invalid("status %q is not one of succeeded, failed, cancelled", status)
	}
	if len(reason) > api.MaxReason {
		return api.Run{}, ErrTooLarge
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, err := m.openTracked(id)
	if err != nil {
		return api.Run{}, err
	}
	m.finish(rec, status, plain(reason))
	if err := m.save(rec); err != nil {
		return api.Run{}, err
	}
	return rec.run.Copy(), nil
}

// openTracked returns a tracked run that is still running. The caller holds m.mu.
func (m *Manager) openTracked(id string) (*record, error) {
	rec, ok := m.records[id]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	if !tracked(rec) {
		return nil, conflict("run %s is not a tracked local run", id)
	}
	if rec.run.Status != api.RunRunning {
		return nil, conflict("run %s is already %s", id, rec.run.Status)
	}
	return rec, nil
}

func hasControl(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

// plain replaces control characters, which a terminal would act on, with
// spaces. Reasons and errors are text for people.
func plain(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
}

// hasCycle reports whether the needs of steps (all of which name declared
// steps) form a cycle, including a step that needs itself.
func hasCycle(steps []api.PlanStep) bool {
	left := make(map[string]int, len(steps))
	users := map[string][]string{}
	for _, s := range steps {
		left[s.ID] = len(s.Needs)
		for _, n := range s.Needs {
			users[n] = append(users[n], s.ID)
		}
	}
	var ready []string
	for id, n := range left {
		if n == 0 {
			ready = append(ready, id)
		}
	}
	done := 0
	for len(ready) > 0 {
		id := ready[len(ready)-1]
		ready = ready[:len(ready)-1]
		done++
		for _, u := range users[id] {
			if left[u]--; left[u] == 0 {
				ready = append(ready, u)
			}
		}
	}
	return done != len(steps)
}
