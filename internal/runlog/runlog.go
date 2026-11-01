package runlog

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

// Log writes timestamped run-log lines to w.
// now is the clock. Timestamps do not move earlier than the previous line.
// A Log is safe for concurrent use: each exported method formats, stamps and
// writes under one lock, so lines from different Steps never interleave
// within a line.
type Log struct {
	mu      sync.Mutex
	w       io.Writer
	now     func() time.Time
	prev    time.Time
	setupAt time.Time
	stepAt  map[string]time.Time
	pending map[string]*bytes.Buffer
}

// New returns a run log that writes to w using now as its clock.
func New(w io.Writer, now func() time.Time) *Log {
	return &Log{
		w:       w,
		now:     now,
		stepAt:  make(map[string]time.Time),
		pending: make(map[string]*bytes.Buffer),
	}
}

// Setup writes the five environment lines before any step.
// The lines name osName, arch, shell, and dir, and no environment variables.
func (l *Log) Setup(osName, arch, shell, dir string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.setupAt = l.stamp()
	l.writeAt(l.setupAt, "setup environment ready")
	l.line("setup os " + osName)
	l.line("setup arch " + arch)
	l.line("setup shell " + shell)
	l.line("setup directory " + dir)
}

// StepStarted writes the start line for id.
func (l *Log) StepStarted(id string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	ts := l.stamp()
	l.stepAt[id] = ts
	l.pending[id] = &bytes.Buffer{}
	l.writeAt(ts, "step "+id+" started")
}

// StepOutput writes one occurrence for each completed output line in chunk.
func (l *Log) StepOutput(id string, chunk []byte) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(chunk) == 0 {
		return
	}
	b := l.buf(id)
	b.Write(chunk)
	for {
		data := b.Bytes()
		i := bytes.IndexByte(data, '\n')
		if i < 0 {
			return
		}
		line := strings.TrimRight(string(data[:i]), "\r")
		b.Next(i + 1)
		l.line("step " + id + " | " + line)
	}
}

// StepFinished writes the step duration and exit code.
// startErr replaces the exit code when the command did not start.
func (l *Log) StepFinished(id string, exitCode *int, startErr error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.flush(id)
	finish := l.stamp()
	dur := formatDuration(finish.Sub(l.stepAt[id]))
	if startErr != nil {
		l.writeAt(finish, fmt.Sprintf("step %s finished duration %s error %s", id, dur, startErr.Error()))
		return
	}
	code := 0
	if exitCode != nil {
		code = *exitCode
	}
	l.writeAt(finish, fmt.Sprintf("step %s finished duration %s exit %d", id, dur, code))
}

// StepStopped records that Step id was stopped because Step by failed.
// It first writes the Step's pending partial output line. The caller still
// calls StepFinished afterwards.
func (l *Log) StepStopped(id, by string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.flush(id)
	l.line("step " + id + " stopped because step " + by + " failed")
}

// SummarySucceeded writes the closing line for a successful run.
// steps is the number of steps that started.
func (l *Log) SummarySucceeded(steps int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.summary("succeeded", steps)
}

// SummaryFailed writes the failed summary and one error line.
// errorText names the pipeline and the failed step, without an "error " prefix.
func (l *Log) SummaryFailed(steps int, errorText string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.summary("failed", steps)
	l.line("error " + errorText)
}

func (l *Log) summary(status string, steps int) {
	finish := l.stamp()
	dur := formatDuration(finish.Sub(l.setupAt))
	l.writeAt(finish, fmt.Sprintf("summary %s steps %d duration %s", status, steps, dur))
}

func (l *Log) buf(id string) *bytes.Buffer {
	b := l.pending[id]
	if b == nil {
		b = &bytes.Buffer{}
		l.pending[id] = b
	}
	return b
}

func (l *Log) flush(id string) {
	b := l.pending[id]
	if b == nil || b.Len() == 0 {
		return
	}
	line := strings.TrimRight(b.String(), "\r")
	b.Reset()
	l.line("step " + id + " | " + line)
}

func (l *Log) line(text string) {
	l.writeAt(l.stamp(), text)
}

func (l *Log) writeAt(ts time.Time, text string) {
	fmt.Fprintf(l.w, "%s %s\n", ts.Format("2006-01-02 15:04:05"), text)
}

func (l *Log) stamp() time.Time {
	now := l.now()
	if !l.prev.IsZero() && now.Before(l.prev) {
		return l.prev
	}
	l.prev = now
	return now
}

func formatDuration(d time.Duration) string {
	ms := d.Round(time.Millisecond) / time.Millisecond
	if ms < 0 {
		ms = 0
	}
	return fmt.Sprintf("%d.%03ds", ms/1000, ms%1000)
}
