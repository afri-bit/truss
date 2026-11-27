package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

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
	s := Sender{Dir: dir, Env: os.Environ()}
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

// Review of spec 030: a command that leaves a child behind is stopped as a
// whole at the deadline.
func TestCommand_IsStoppedWithItsChildrenAtTheDeadline(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	old := Timeout
	Timeout = time.Second
	defer func() { Timeout = old }()
	marker := filepath.Join(t.TempDir(), "survived")
	s := Sender{Dir: t.TempDir(), Env: os.Environ()}
	start := time.Now()
	errs := s.Send(context.Background(), &pipeline.Notify{Run: "(sleep 3; touch \"" + marker + "\") & sleep 30"}, Outcome{})
	if took := time.Since(start); took > 8*time.Second || len(errs) != 1 {
		t.Fatalf("took %s, errs %v", took, errs)
	}
	time.Sleep(3 * time.Second)
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("a child of the notify command outlived it")
	}
}

func TestWebhook_RedirectsAreRefusedAndRetriesHappenOnce(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch r.URL.Path {
		case "/redirect":
			http.Redirect(w, r, "http://example.invalid/steal", http.StatusTemporaryRedirect)
		case "/flaky":
			w.WriteHeader(http.StatusBadGateway)
		}
	}))
	defer srv.Close()
	s := Sender{HTTP: srv.Client(), Secret: func(name string) (string, bool) { return srv.URL + "/" + name, true }}
	errs := s.Send(context.Background(), &pipeline.Notify{Webhook: &pipeline.Webhook{URL: "secret:redirect"}}, Outcome{})
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "a redirect") || calls != 1 {
		t.Fatalf("redirect: %v, calls %d", errs, calls)
	}
	calls = 0
	errs = s.Send(context.Background(), &pipeline.Notify{Webhook: &pipeline.Webhook{URL: "secret:flaky"}}, Outcome{})
	if len(errs) != 1 || calls != 2 {
		t.Fatalf("flaky: %v, calls %d", errs, calls)
	}
}

func TestBody_ChatTextCannotMentionOrLink(t *testing.T) {
	o := Outcome{Flow: "x", Status: "failed", Reason: "<!channel> @everyone <https://a|b>", RunID: "r"}
	slack, _ := Body("slack", o)
	var m map[string]string
	if err := json.Unmarshal(slack, &m); err != nil || strings.Contains(m["text"], "<!channel>") || !strings.Contains(m["text"], "&lt;!channel&gt;") {
		t.Fatalf("slack = %s", slack)
	}
	discord, _ := Body("discord", o)
	if !strings.Contains(string(discord), `"allowed_mentions":{"parse":[]}`) {
		t.Fatalf("discord = %s", discord)
	}
	teams, _ := Body("teams", o)
	if !strings.Contains(string(teams), `"text":`) {
		t.Fatalf("teams = %s", teams)
	}
}

// The desktop texts are arguments or environment, never code.
func TestDesktop_TextsAreData(t *testing.T) {
	var name string
	var args, env []string
	s := Sender{Command: func(ctx context.Context, n string, a ...string) *exec.Cmd {
		name, args = n, a
		c := exec.CommandContext(ctx, os.Args[0], "-test.run=^$")
		return c
	}}
	o := Outcome{Flow: `-x"; do shell script "id`, Status: "failed", RunID: "r", Trigger: "manual"}
	_ = s.desktop(context.Background(), o)
	switch runtime.GOOS {
	case "linux":
		if name != "notify-send" || args[len(args)-3] != "--" || args[len(args)-2] != Title(o) {
			t.Fatalf("notify-send %v", args)
		}
	case "darwin":
		if name != "osascript" || args[len(args)-2] != Title(o) || !strings.Contains(strings.Join(args, " "), "on run argv") {
			t.Fatalf("osascript %v", args)
		}
	case "windows":
		if !strings.HasSuffix(name, "powershell.exe") || args[len(args)-2] != "-EncodedCommand" {
			t.Fatalf("powershell %v", args)
		}
		_ = env
	}
}
