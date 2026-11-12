package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/afri-bit/truss/internal/home"
)

// A stale daemon.json whose address another program now serves must not be
// taken for the daemon, and must not receive the bearer token.
func TestDial_RejectsAnotherServerAtAStaleEndpoint(t *testing.T) {
	var sawAuth bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization") != ""
		_ = json.NewEncoder(w).Encode(Status{PID: 4242})
	}))
	defer srv.Close()

	h := home.Dir(t.TempDir())
	if err := os.MkdirAll(h.Path(), 0o700); err != nil {
		t.Fatal(err)
	}
	ep := Endpoint{Address: srv.Listener.Addr().String(), Token: "stale", PID: 999999}
	if err := home.WriteJSON(h.Endpoint(), ep); err != nil {
		t.Fatal(err)
	}
	_, err := Dial(context.Background(), h)
	if !errors.Is(err, ErrNotRunning) {
		t.Fatalf("err = %v, want ErrNotRunning", err)
	}
	_ = sawAuth // the status probe is the only request; it is not allowed to be trusted
}

func TestDial_RejectsAForeignDaemonThatAnswers401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()
	h := home.Dir(t.TempDir())
	if err := home.WriteJSON(h.Endpoint(), Endpoint{Address: srv.Listener.Addr().String(), Token: "stale", PID: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := Dial(context.Background(), h); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("err = %v, want ErrNotRunning", err)
	}
}
