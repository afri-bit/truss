package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/afri-bit/truss/internal/webui"
)

// demoHandler serves TRUSS Desktop without a daemon, for `truss desktop --demo`.
// The page then runs on sample data it generates itself; this handler has no API
// and answers every /api/ request with 404. It sends the same security headers
// as the daemon and, like the daemon, answers only requests addressed to this machine.
func demoHandler() http.Handler {
	files := http.FileServerFS(webui.Files())
	demoFiles := http.FileServerFS(webui.DemoFiles())
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !loopbackHost(r.Host) {
			http.Error(w, "this demo server only answers requests addressed to this machine", http.StatusForbidden)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; frame-ancestors 'none'")
		w.Header().Set("Cache-Control", "no-store")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"error":"demo mode has no daemon"}`+"\n")
			return
		}
		if r.URL.Path == "/demo.js" { // only this server has the sample data
			demoFiles.ServeHTTP(w, r)
			return
		}
		files.ServeHTTP(w, r)
	})
}

// loopbackHost reports whether a Host header names this machine, so a page on
// another site cannot reach the server through DNS rebinding.
func loopbackHost(hostport string) bool {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		host = hostport
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// desktopDemo serves TRUSS Desktop on a loopback port with sample data and no
// daemon, until ctx ends (Ctrl-C). It is a design and review aid: nothing it
// shows comes from a daemon, and the page says so.
func (in invocation) desktopDemo(ctx context.Context, open bool) int {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return in.fail(fmt.Errorf("start the demo server: %w", err))
	}
	srv := &http.Server{Handler: demoHandler(), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, IdleTimeout: 2 * time.Minute}
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ln) }()
	url := "http://" + ln.Addr().String() + "/#demo"
	fmt.Fprintf(in.stdout, "TRUSS Desktop (demo, sample data, no daemon): %s\n", url)
	fmt.Fprintln(in.stdout, "Press Ctrl-C to stop.")
	if open {
		if err := openBrowser(url); err != nil {
			fmt.Fprintf(in.stderr, "could not open a browser (%s); open the address above yourself\n", err.Error())
		}
	}
	select {
	case <-ctx.Done():
	case err := <-done:
		return in.fail(fmt.Errorf("demo server: %w", err))
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdown); err != nil {
		if !errors.Is(err, context.DeadlineExceeded) {
			return in.fail(fmt.Errorf("stop the demo server: %w", err))
		}
		_ = srv.Close() // connections still open after the grace period are cut
	}
	return 0
}
