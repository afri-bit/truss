package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/afri-bit/truss/internal/executor"
	"github.com/afri-bit/truss/internal/pipeline"
)

func TestWanted(t *testing.T) {
	failed := Outcome{Status: "failed"}
	ok := Outcome{Status: "succeeded"}
	recovered := Outcome{Status: "succeeded", Previous: "failed"}
	missed := Outcome{Status: "skipped", Missed: true}
	skipped := Outcome{Status: "skipped"}
	for _, c := range []struct {
		on   []string
		o    Outcome
		want bool
	}{
		{nil, failed, true}, {nil, ok, false}, {nil, missed, true},
		{[]string{"success"}, ok, true}, {[]string{"success"}, failed, false},
		{[]string{"recovered"}, recovered, true}, {[]string{"recovered"}, ok, false},
		{[]string{"changed"}, recovered, true}, {[]string{"changed"}, Outcome{Status: "failed", Previous: "failed"}, false},
		{[]string{"always"}, ok, true}, {[]string{"always"}, skipped, false},
	} {
		n := &pipeline.Notify{On: c.on, Desktop: true}
		if got := Wanted(n, c.o); got != c.want {
			t.Errorf("Wanted(%v, %+v) = %v", c.on, c.o, got)
		}
	}
	if Wanted(nil, failed) {
		t.Error("no notify block wants nothing")
	}
}

func TestWebhook_FormatsAndSecretURL(t *testing.T) {
	var got []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var m map[string]any
		data, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(data, &m)
		got = append(got, m)
	}))
	defer srv.Close()
	o := Outcome{Flow: "backup", RunID: "r1", Status: "failed", Reason: "backup failed at copy (exit 2)", Trigger: "schedule"}
	s := Sender{HTTP: srv.Client(), Secret: func(name string) (string, bool) { return srv.URL, name == "HOOK" }}
	for _, format := range []string{"slack", "discord", "generic"} {
		n := &pipeline.Notify{Webhook: &pipeline.Webhook{URL: "secret:HOOK", Format: format}}
		if errs := s.Send(context.Background(), n, o); len(errs) != 0 {
			t.Fatalf("%s: %v", format, errs)
		}
	}
	if len(got) != 3 || !strings.Contains(got[0]["text"].(string), "TRUSS: backup failed") ||
		got[1]["content"] == nil || got[2]["flow"] != "backup" || got[2]["status"] != "failed" {
		t.Fatalf("bodies = %v", got)
	}
	// A missing secret is reported by name, never a URL.
	n := &pipeline.Notify{Webhook: &pipeline.Webhook{URL: "secret:NOPE", Format: "slack"}}
	if errs := s.Send(context.Background(), n, o); len(errs) != 1 || !strings.Contains(errs[0].Error(), "the secret NOPE is not set") {
		t.Fatalf("errs = %v", errs)
	}
}

func TestWebhook_AClientErrorIsReportedWithoutTheURL(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	s := Sender{HTTP: srv.Client(), Secret: func(string) (string, bool) { return srv.URL + "/token-abc", true }}
	n := &pipeline.Notify{Webhook: &pipeline.Webhook{URL: "secret:HOOK"}}
	errs := s.Send(context.Background(), n, Outcome{Status: "failed"})
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "answered 403") || strings.Contains(errs[0].Error(), "token-abc") || calls != 1 {
		t.Fatalf("errs = %v, calls = %d", errs, calls)
	}
}

func TestCommand_GetsTheOutcome(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out.txt")
	run := "echo $TRUSS_NOTIFY_FLOW $TRUSS_NOTIFY_STATUS > \"" + out + "\""
	if runtime.GOOS == "windows" {
		run = "echo %TRUSS_NOTIFY_FLOW% %TRUSS_NOTIFY_STATUS%> \"" + out + "\""
	}
	s := Sender{Shell: executor.ShellCommand, Dir: dir, Env: os.Environ()}
	if errs := s.Send(context.Background(), &pipeline.Notify{Run: run}, Outcome{Flow: "backup", Status: "failed"}); len(errs) != 0 {
		t.Fatal(errs)
	}
	data, err := os.ReadFile(out)
	if err != nil || strings.TrimSpace(string(data)) != "backup failed" {
		t.Fatalf("out = %q, %v", data, err)
	}
	if errs := s.Send(context.Background(), &pipeline.Notify{Run: "exit 3"}, Outcome{}); len(errs) != 1 {
		t.Fatalf("a failing command was not reported: %v", errs)
	}
}
