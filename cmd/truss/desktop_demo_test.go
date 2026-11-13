package main

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/afri-bit/truss/internal/webui"
)

func TestDemoHandler_ServesThePageWithTheDaemonsHeaders(t *testing.T) {
	rec := httptest.NewRecorder()
	demoHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://127.0.0.1:9/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	for _, want := range []string{`src="app.js"`} {
		if !strings.Contains(string(body), want) {
			t.Errorf("index.html lacks %s", want)
		}
	}
	if got := rec.Header().Get("Content-Security-Policy"); got != "default-src 'self'; frame-ancestors 'none'" {
		t.Errorf("CSP = %q", got)
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("nosniff = %q", got)
	}
}

func TestDemoHandler_ServesTheSampleData(t *testing.T) {
	rec := httptest.NewRecorder()
	demoHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://127.0.0.1:9/demo.js", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "window.TrussDemo") {
		t.Fatalf("GET /demo.js: %d %.80q", rec.Code, rec.Body.String())
	}
}

func TestDaemonFiles_HaveNoSampleData(t *testing.T) {
	// The daemon serves webui.Files(); demo.js must not be among them, so #demo
	// on a real daemon's page cannot switch to sample data.
	if _, err := fs.Stat(webui.Files(), "demo.js"); err == nil {
		t.Fatal("webui.Files() contains demo.js")
	}
}

func TestDemoHandler_HasNoAPI(t *testing.T) {
	rec := httptest.NewRecorder()
	demoHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://localhost:9/api/v1/status", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "demo mode has no daemon") {
		t.Errorf("body = %q", rec.Body.String())
	}
}

func TestDemoHandler_RefusesOtherHosts(t *testing.T) {
	for _, host := range []string{"evil.example", "evil.example:80", "10.0.0.1:9"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Host = host
		demoHandler().ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("Host %s: status = %d, want 403", host, rec.Code)
		}
	}
}

func TestDesktopDemo_ServesWithoutADaemonUntilCancelled(t *testing.T) {
	s := newSession(t, false) // no daemon: --demo must not need one
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out, w := io.Pipe()
	var stderr bytes.Buffer
	code := make(chan int, 1)
	go func() {
		code <- Run(ctx, []string{"desktop", "--demo", "--no-open"}, s.dir, s.env, w, &stderr)
		w.Close()
	}()
	line, err := bufio.NewReader(out).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`^TRUSS Desktop \(demo, sample data, no daemon\): (http://127\.0\.0\.1:\d+/)#demo\n$`).FindStringSubmatch(line)
	if m == nil {
		t.Fatalf("first line = %q", line)
	}
	go func() { _, _ = io.Copy(io.Discard, out) }()
	res, err := http.Get(m[1])
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: %d", m[1], res.StatusCode)
	}
	cancel()
	select {
	case c := <-code:
		if c != 0 {
			t.Fatalf("exit %d, stderr %q", c, stderr.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the demo server did not stop after cancel")
	}
}
