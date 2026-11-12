//go:build linux

package main

import (
	"bytes"
	"context"
	"os"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/afri-bit/truss/internal/home"
	"github.com/afri-bit/truss/internal/secrets"
)

// secretStore opens the store of the session's TRUSS home, to read back what a
// command stored.
func secretStore(s *session) *secrets.Store {
	h, _ := home.Resolve(s.env)
	return secrets.Open(h.Secrets())
}

// syncBuffer is a bytes.Buffer that several goroutines may use.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// openPTY returns the two ends of a new pseudo-terminal.
func openPTY(t *testing.T) (master, slave *os.File) {
	t.Helper()
	m, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		t.Skipf("no pseudo-terminal here: %v", err)
	}
	var unlock int32
	var n uint32
	// #nosec G103 -- ioctl arguments for a test-only pseudo-terminal.
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, m.Fd(), syscall.TIOCSPTLCK, uintptr(unsafe.Pointer(&unlock))); e != 0 {
		t.Fatal(e)
	}
	// #nosec G103 -- ioctl arguments for a test-only pseudo-terminal.
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, m.Fd(), syscall.TIOCGPTN, uintptr(unsafe.Pointer(&n))); e != 0 {
		t.Fatal(e)
	}
	s, err := os.OpenFile("/dev/pts/"+itoa(n), os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close(); _ = m.Close() })
	return m, s
}

func itoa(n uint32) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	for ; n > 0; n /= 10 {
		d = append([]byte{byte('0' + n%10)}, d...)
	}
	return string(d)
}

func waitFor(t *testing.T, b *syncBuffer, text string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(b.String(), text) {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %q in %q", text, b.String())
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func echoOn(t *testing.T, f *os.File) bool {
	t.Helper()
	var tm syscall.Termios
	if err := ioctl(f.Fd(), getTermios, &tm); err != nil {
		t.Fatal(err)
	}
	return tm.Lflag&syscall.ECHO != 0
}

// promptSession runs `secret set NAME` with a pseudo-terminal as stdin and
// returns what the terminal showed (the master's output) and the command's
// stderr, plus a function that types a line.
type promptSession struct {
	slave  *os.File
	stderr *syncBuffer
	shown  *syncBuffer
	type_  func(string)
	done   chan int
	stdout *syncBuffer
}

func startPrompt(t *testing.T, ctx context.Context, s *session, name string) *promptSession {
	t.Helper()
	master, slave := openPTY(t)
	p := &promptSession{slave: slave, stderr: &syncBuffer{}, shown: &syncBuffer{}, stdout: &syncBuffer{}, done: make(chan int, 1)}
	go func() { // everything the terminal echoes or prints
		buf := make([]byte, 4096)
		for {
			n, err := master.Read(buf)
			p.shown.Write(buf[:n])
			if err != nil {
				return
			}
		}
	}()
	p.type_ = func(line string) { _, _ = master.WriteString(line) }
	go func() {
		p.done <- RunWithInput(ctx, []string{"secret", "set", name}, s.dir, s.env, slave, p.stdout, p.stderr)
	}()
	return p
}

func TestSecretSet_HiddenPromptStoresTheValueWithoutEcho(t *testing.T) {
	s := newSession(t, false)
	p := startPrompt(t, context.Background(), s, "PROMPTED")

	waitFor(t, p.stderr, "Value for PROMPTED")
	if echoOn(t, p.slave) {
		t.Error("echo is on while the prompt waits")
	}
	p.type_(canary + "\n")
	waitFor(t, p.stderr, "Repeat the value")
	p.type_(canary + "\n")
	if code := <-p.done; code != 0 {
		t.Fatalf("exit %d, stderr %q", code, p.stderr.String())
	}
	if strings.Contains(p.shown.String(), canary) || strings.Contains(p.stderr.String(), canary) {
		t.Errorf("the value was shown: %q / %q", p.shown.String(), p.stderr.String())
	}
	if !echoOn(t, p.slave) {
		t.Error("echo was not restored")
	}
	all, err := (secretStore(s)).Load()
	if err != nil || all["PROMPTED"] != canary {
		t.Errorf("stored = %v, %v", all, err)
	}
}

func TestSecretSet_HiddenPromptMismatchStoresNothing(t *testing.T) {
	s := newSession(t, false)
	p := startPrompt(t, context.Background(), s, "PROMPTED")
	waitFor(t, p.stderr, "Value for PROMPTED")
	p.type_(canary + "\n")
	waitFor(t, p.stderr, "Repeat the value")
	p.type_(canary + "-different\n")
	if code := <-p.done; code != 1 || !strings.Contains(p.stderr.String(), "differ") {
		t.Fatalf("exit %d, stderr %q", code, p.stderr.String())
	}
	if got := s.ok("secret", "list"); got != "" {
		t.Errorf("stored %q", got)
	}
}

func TestSecretSet_InterruptEndsThePromptAndRestoresEcho(t *testing.T) {
	s := newSession(t, false)
	ctx, cancel := context.WithCancel(context.Background())
	p := startPrompt(t, ctx, s, "PROMPTED")
	waitFor(t, p.stderr, "Value for PROMPTED")
	cancel()
	select {
	case code := <-p.done:
		if code != 1 {
			t.Errorf("exit %d", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the prompt did not end after the interrupt")
	}
	if !echoOn(t, p.slave) {
		t.Error("echo was not restored")
	}
}
