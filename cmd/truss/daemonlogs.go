package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/afri-bit/truss/internal/api"
	"github.com/afri-bit/truss/internal/home"
)

const (
	// defaultLogLines is how many lines `daemon logs` prints.
	defaultLogLines = 50
	// maxTailBytes is the most of the daemon log read to find its last lines.
	maxTailBytes = 1 << 20
)

// daemonStatusDoc is the JSON of `truss daemon status --json`.
type daemonStatusDoc struct {
	Running bool        `json:"running"`
	Daemon  *api.Status `json:"daemon"`
}

// daemonStatusJSON is `daemon status` for scripts. Like the text form it exits
// 1 when the daemon does not run.
func (in invocation) daemonStatusJSON(ctx context.Context) int {
	c, err := in.dial(ctx)
	if errors.Is(err, api.ErrNotRunning) {
		writeJSON(in.stdout, daemonStatusDoc{})
		return 1
	}
	if err != nil {
		return in.fail(err)
	}
	st, err := c.Status(ctx)
	if err != nil {
		return in.fail(err)
	}
	writeJSON(in.stdout, daemonStatusDoc{Running: true, Daemon: &st})
	return 0
}

// daemonLogs prints the end of the daemon's own log file, which holds its
// start, stop and scheduling messages, and with --follow keeps printing. It
// reads a file, so it works when the daemon is stopped, which is when the
// log is most wanted.
func (in invocation) daemonLogs(ctx context.Context, args []string) int {
	pos, flags, err := options(args, []string{"lines", "n"}, []string{"follow", "f"})
	if err != nil || len(pos) != 0 {
		return in.usage()
	}
	lines := defaultLogLines
	for _, name := range []string{"lines", "n"} {
		if text, ok := flags[name]; ok {
			if lines, err = positive(text); err != nil {
				return in.fail(fmt.Errorf("--lines must be a whole number of at least 1, got %q", text))
			}
		}
	}
	h, err := home.Resolve(in.env)
	if err != nil {
		return in.fail(err)
	}
	follow := flags["follow"] != "" || flags["f"] != ""
	f, err := openLog(h.DaemonLog())
	if errors.Is(err, os.ErrNotExist) {
		if !follow {
			fmt.Fprintf(in.stdout, "no daemon log yet (%s)\n", h.DaemonLog())
			return 0
		}
		return in.fail(fmt.Errorf("no daemon log yet (%s); start the daemon first", h.DaemonLog()))
	}
	if err != nil {
		return in.fail(err)
	}
	defer func() { _ = f.Close() }()
	offset, err := printTail(in.stdout, f, lines)
	if err != nil {
		return in.fail(err)
	}
	for follow {
		select {
		case <-ctx.Done():
			return 0
		case <-time.After(followEvery):
		}
		if now, err := os.Stat(h.DaemonLog()); err == nil {
			if open, err := f.Stat(); err == nil && !os.SameFile(open, now) { // the log was replaced
				next, err := openLog(h.DaemonLog())
				if err != nil {
					return in.fail(err)
				}
				_ = f.Close()
				f, offset = next, 0
			}
		}
		info, err := f.Stat()
		if err != nil {
			return in.fail(err)
		}
		if info.Size() < offset { // the log was replaced by a shorter one
			offset = 0
		}
		if info.Size() == offset {
			continue
		}
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			return in.fail(err)
		}
		n, err := io.Copy(in.stdout, io.LimitReader(f, info.Size()-offset))
		if err != nil {
			return in.fail(err)
		}
		offset += n
	}
	return 0
}

// openLog opens the daemon log, which must be a regular file. Opening a named
// pipe would block for good, and Ctrl-C cannot reach a process stuck in open.
func openLog(path string) (*os.File, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", path)
	}
	f, err := os.OpenFile(path, os.O_RDONLY|openNoBlock, 0) // #nosec G304 -- the daemon log inside the TRUSS home, checked above
	if err != nil {
		return nil, err
	}
	if info, err := f.Stat(); err != nil || !info.Mode().IsRegular() {
		_ = f.Close()
		return nil, fmt.Errorf("%s is not a regular file", path)
	}
	return f, nil
}

// printTail writes the last n lines of f and returns the offset it read to.
func printTail(w io.Writer, f *os.File, n int) (int64, error) {
	info, err := f.Stat()
	if err != nil {
		return 0, err
	}
	size := info.Size()
	start := max(size-maxTailBytes, 0)
	buf := make([]byte, size-start)
	if _, err := f.ReadAt(buf, start); err != nil && !errors.Is(err, io.EOF) {
		return 0, err
	}
	if start > 0 { // began inside a line: drop the cut piece
		if i := bytes.IndexByte(buf, '\n'); i >= 0 {
			buf = buf[i+1:]
		}
	}
	buf = bytes.TrimRight(buf, "\n")
	cut := len(buf)
	for seen := 0; cut > 0; cut-- {
		if buf[cut-1] == '\n' {
			seen++
			if seen == n {
				break
			}
		}
	}
	if len(buf) > 0 {
		_, err = fmt.Fprintf(w, "%s\n", buf[cut:])
	}
	return size, err
}
