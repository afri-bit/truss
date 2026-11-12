package secrets

import (
	"bytes"
	"sort"
	"strings"
	"sync"
)

// Mask replaces a secret value wherever one is found.
const Mask = "***"

// Redactor removes secret values from a stream of output. Write may be
// called with the stream cut anywhere, even inside a value: a tail that could
// still become a value is held back until the next Write or Flush.
//
// A Redactor is safe for concurrent use. A nil Redactor passes everything
// through unchanged.
type Redactor struct {
	mu      sync.Mutex
	values  [][]byte // longest first
	pending []byte
	out     func([]byte)
}

// NewRedactor returns a Redactor for the given values, writing redacted
// output through out. Empty values are ignored. It returns nil when there is
// nothing to redact, so callers can skip the work.
func NewRedactor(values []string, out func([]byte)) *Redactor {
	var vs [][]byte
	seen := map[string]bool{}
	for _, v := range values {
		if v != "" && !seen[v] {
			seen[v] = true
			vs = append(vs, []byte(v))
		}
	}
	if len(vs) == 0 {
		return nil
	}
	sort.SliceStable(vs, func(i, j int) bool { return len(vs[i]) > len(vs[j]) })
	return &Redactor{values: vs, out: out}
}

// Write redacts p and passes what is safe to release to the output.
func (r *Redactor) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pending = append(r.pending, p...)
	r.drain(false)
	return len(p), nil
}

// Flush releases held-back output. Call it when the stream ends.
func (r *Redactor) Flush() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.drain(true)
}

// drain scans pending once. At each position a held-back prefix of a longer
// value wins, then the longest full match is masked, otherwise the byte passes.
func (r *Redactor) drain(final bool) {
	var out bytes.Buffer
	buf := r.pending
	i := 0
	for i < len(buf) {
		rest := buf[i:]
		if !final && r.couldGrow(rest) {
			break
		}
		if n := r.match(rest); n > 0 {
			out.WriteString(Mask)
			i += n
			continue
		}
		out.WriteByte(buf[i])
		i++
	}
	r.pending = append([]byte(nil), buf[i:]...)
	if out.Len() > 0 {
		r.out(out.Bytes())
	}
}

// couldGrow reports whether rest is a proper prefix of some value, so more
// output might still complete it.
func (r *Redactor) couldGrow(rest []byte) bool {
	for _, v := range r.values {
		if len(rest) < len(v) && bytes.HasPrefix(v, rest) {
			return true
		}
	}
	return false
}

// match returns the length of the longest value that starts rest, or 0.
func (r *Redactor) match(rest []byte) int {
	for _, v := range r.values {
		if bytes.HasPrefix(rest, v) {
			return len(v)
		}
	}
	return 0
}

// Scrub returns s with every value replaced. It is for whole strings; use a
// Redactor for streams.
func Scrub(s string, values []string) string {
	var pairs []string
	vs := append([]string(nil), values...)
	sort.SliceStable(vs, func(i, j int) bool { return len(vs[i]) > len(vs[j]) })
	for _, v := range vs {
		if v != "" {
			pairs = append(pairs, v, Mask)
		}
	}
	if len(pairs) == 0 {
		return s
	}
	return strings.NewReplacer(pairs...).Replace(s)
}
