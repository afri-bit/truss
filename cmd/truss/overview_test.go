package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/afri-bit/truss/internal/api"
	"github.com/afri-bit/truss/internal/project"
)

// Spec 022: the read-only commands that answer "what is going on?".

const demoFlow = `version: "1"
pipeline: {name: demo, concurrency: 2}
env: {GREETING: very-private-value}
on:
  schedule:
    - cron: "0 9 * * 1-5"
      timezone: UTC
steps:
  - {id: lint, run: "echo lint"}
  - {id: build, run: "echo build"}
  - {id: test, run: "exit 3", needs: [lint, build], retries: 1}
`

const okFlow = `version: "1"
pipeline: {name: ok}
steps:
  - {id: hello, run: "echo hello"}
`

func projectSession(t *testing.T, withDaemon bool) *session {
	t.Helper()
	s := newSession(t, withDaemon)
	s.write("truss.yaml", projectFile)
	s.write(".truss/flows/demo.yaml", demoFlow)
	s.write(".truss/flows/ok.yaml", okFlow)
	return s
}

func decode[T any](t *testing.T, text string) T {
	t.Helper()
	var v T
	if err := json.Unmarshal([]byte(text), &v); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, text)
	}
	return v
}

func TestStatus_WithoutADaemon(t *testing.T) {
	s := projectSession(t, false)
	out := s.ok("status")
	for _, want := range []string{"TRUSS", "Project    demo, 2 flows", "Daemon     not running (start it with: truss daemon start)"} {
		if !strings.Contains(out, want) {
			t.Errorf("status lacks %q:\n%s", want, out)
		}
	}
	doc := decode[statusDoc](t, s.ok("status", "--json"))
	if doc.Daemon != nil || doc.DesktopURL != nil || doc.Project == nil || doc.Project.Flows != 2 || doc.RecentRuns == nil {
		t.Fatalf("status json = %+v", doc)
	}
}

func TestStatus_NoProjectIsNotAnError(t *testing.T) {
	s := newSession(t, false)
	if out := s.ok("status"); !strings.Contains(out, "Project    none here") {
		t.Fatalf("status = %q", out)
	}
}

func TestStatus_WithADaemonAndARun(t *testing.T) {
	s := projectSession(t, true)
	s.ok("register", "--all")
	s.truss("trigger", "demo", "--follow")
	out := s.ok("status")
	for _, want := range []string{"Daemon     running, pid ", "Flows      2 registered: 2 ready", "Last run   demo failed", "truss runs show "} {
		if !strings.Contains(out, want) {
			t.Errorf("status lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "token") {
		t.Errorf("status shows the Desktop token:\n%s", out)
	}
	doc := decode[statusDoc](t, s.ok("status", "--json"))
	if doc.Daemon == nil || doc.Pipelines.Registered != 2 || len(doc.RecentRuns) != 1 || doc.DesktopURL == nil || !strings.HasPrefix(*doc.DesktopURL, "http://127.0.0.1:") {
		t.Fatalf("status json = %+v", doc)
	}
}

func TestFlows_ListShowGraphWithoutADaemon(t *testing.T) {
	s := projectSession(t, false)
	list := s.ok("flows")
	for _, want := range []string{"FLOW", "demo  3", "ok    1", "unknown", "daemon is not running"} {
		if !strings.Contains(list, want) {
			t.Errorf("flows lacks %q:\n%s", want, list)
		}
	}
	if s.ok("flows", "list") != list {
		t.Error("`flows list` differs from `flows`")
	}
	show := s.ok("flows", "show", "demo")
	for _, want := range []string{"Concurrency   2", "Environment   GREETING (names only)", "0 9 * * 1-5 (UTC, missed: skip)", "test", "lint,build"} {
		if !strings.Contains(show, want) {
			t.Errorf("flows show lacks %q:\n%s", want, show)
		}
	}
	if strings.Contains(show, "very-private-value") {
		t.Errorf("flows show prints an environment value:\n%s", show)
	}
	graph := s.ok("flows", "graph", "demo")
	want := "demo: 3 steps in 2 stages, at most 2 at once\n\nstage 1\n  [lint]\n  [build]\n\nstage 2\n  [test]  <- lint, build\n"
	if graph != want {
		t.Fatalf("graph =\n%s\nwant\n%s", graph, want)
	}
}

func TestFlows_JSONHidesEnvironmentValues(t *testing.T) {
	s := projectSession(t, false)
	raw := s.ok("flows", "show", "demo", "--json")
	if strings.Contains(raw, "very-private-value") {
		t.Fatalf("json prints an environment value:\n%s", raw)
	}
	d := decode[flowDoc](t, raw)
	if !d.Valid || d.Name != "demo" || len(d.Steps) != 3 || d.Steps[2].Retries != 1 || d.Env[0] != "GREETING" || d.Daemon != regUnknown {
		t.Fatalf("flow = %+v", d)
	}
	all := decode[[]flowDoc](t, s.ok("flows", "--json"))
	if len(all) != 2 {
		t.Fatalf("flows = %+v", all)
	}
}

func TestFlows_ABrokenFlowDoesNotHideTheOthers(t *testing.T) {
	s := projectSession(t, false)
	s.write(".truss/flows/bad.yaml", "version: \"1\"\npipeline: {name: bad}\nsteps: []\n")
	out := s.ok("flows")
	if !strings.Contains(out, "INVALID: pipeline must contain one or more steps") || !strings.Contains(out, "demo") {
		t.Fatalf("flows =\n%s", out)
	}
	if code, _, errOut := s.truss("flows", "show", "bad"); code != 1 || !strings.Contains(errOut, "bad.yaml: pipeline must contain") {
		t.Fatalf("show bad: %d %q", code, errOut)
	}
	// The JSON says why and the exit code says it too.
	code, out, _ := s.truss("flows", "show", "bad", "--json")
	if d := decode[flowDoc](t, out); code != 1 || d.Valid || !strings.Contains(d.Error, "one or more steps") {
		t.Fatalf("show bad --json: %d %+v", code, d)
	}
}

func TestFlows_ShowRegistration(t *testing.T) {
	s := projectSession(t, true)
	s.ok("register", "demo")
	s.ok("register", "ok")
	s.ok("pause", "demo")
	d := decode[[]flowDoc](t, s.ok("flows", "--json"))
	if d[0].Name != "demo" || d[0].Daemon != regPaused || d[1].Daemon != regRegistered {
		t.Fatalf("flows = %+v", d)
	}
	s.ok("unregister", "ok")
	if out := s.ok("flows"); !strings.Contains(out, regNotRegistered) {
		t.Fatalf("flows =\n%s", out)
	}
}

func TestFlows_NeedAProjectAndKnowTheirCommands(t *testing.T) {
	s := newSession(t, false)
	if code, _, errOut := s.truss("flows"); code != 1 || !strings.Contains(errOut, "truss.yaml") {
		t.Fatalf("no project: %d %q", code, errOut)
	}
	for _, args := range [][]string{{"flows", "nope"}, {"flows", "show"}, {"flows", "graph", "a", "b"}, {"flows", "list", "x"}, {"flows", "--nope"}} {
		if code, out, errOut := s.truss(args...); code != 1 || errOut != usageText || out != "" {
			t.Errorf("truss %v: %d %q %q", args, code, out, errOut)
		}
	}
	s.write("truss.yaml", projectFile)
	if code, _, errOut := s.truss("flows", "show", "missing"); code != 1 || !strings.Contains(errOut, "missing") {
		t.Fatalf("missing: %d %q", code, errOut)
	}
}

func TestProject_ShowsTheProject(t *testing.T) {
	s := projectSession(t, false)
	out := s.ok("project")
	if !strings.Contains(out, "Project       demo") || !strings.Contains(out, "Flows         2") || !strings.Contains(out, "unknown") {
		t.Fatalf("project =\n%s", out)
	}
	d := decode[map[string]any](t, s.ok("project", "show", "--json"))
	if d["name"] != "demo" || d["flows"] != float64(2) || d["registered"] != nil {
		t.Fatalf("project json = %v", d)
	}
	if code, _, errOut := newSession(t, false).truss("project"); code != 1 || !strings.Contains(errOut, "truss.yaml") {
		t.Fatalf("no project: %d %q", code, errOut)
	}
}

func TestProject_CountsRegisteredFlows(t *testing.T) {
	s := projectSession(t, true)
	s.ok("register", "demo")
	if out := s.ok("project"); !strings.Contains(out, "Registered    1 of 2") {
		t.Fatalf("project =\n%s", out)
	}
	if d := decode[map[string]any](t, s.ok("project", "--json")); d["registered"] != float64(1) {
		t.Fatalf("project json = %v", d)
	}
}

func TestProjects_GroupRegistrationsByProject(t *testing.T) {
	s := projectSession(t, true)
	s.ok("register", "--all")
	s.write("loose/x.yaml", okFlow)
	s.ok("register", "loose/x.yaml", "--name", "loose")
	s.ok("pause", "demo")
	out := s.ok("projects")
	if !strings.Contains(out, "demo          2") || !strings.Contains(out, "(no project)  1") {
		t.Fatalf("projects =\n%s", out)
	}
	groups := decode[[]projectGroup](t, s.ok("projects", "--json"))
	if len(groups) != 2 || groups[0].Name != "demo" || groups[0].Registered != 2 || groups[0].Paused != 1 || groups[1].Root != "" {
		t.Fatalf("groups = %+v", groups)
	}
	if code, _, errOut := newSession(t, false).truss("projects"); code != 1 || !strings.Contains(errOut, "truss daemon start") {
		t.Fatalf("no daemon: %d %q", code, errOut)
	}
	empty := newSession(t, true)
	if out := empty.ok("projects"); !strings.Contains(out, "no projects") {
		t.Fatalf("empty = %q", out)
	}
}

func TestSchedules_ListSoonestFirst(t *testing.T) {
	s := projectSession(t, true)
	s.write(".truss/flows/often.yaml", "version: \"1\"\npipeline: {name: often}\non: {schedule: [{cron: \"* * * * *\", missed: run}]}\nsteps: [{id: a, run: echo}]\n")
	if out := s.ok("schedules"); !strings.Contains(out, "no schedules") {
		t.Fatalf("empty = %q", out)
	}
	s.ok("register", "--all")
	s.ok("pause", "demo")
	rows := decode[[]scheduleRow](t, s.ok("schedules", "--json"))
	if len(rows) != 2 || rows[0].Pipeline != "often" || rows[0].Missed != "run" || rows[1].Pipeline != "demo" || !rows[1].Paused || rows[1].Timezone != "UTC" {
		t.Fatalf("rows = %+v", rows)
	}
	out := s.ok("schedules")
	if !strings.Contains(out, "paused") || !strings.Contains(out, "* * * * *") {
		t.Fatalf("schedules =\n%s", out)
	}
}

func TestRuns_ShowFilterAndLimit(t *testing.T) {
	s := projectSession(t, true)
	s.ok("register", "--all")
	s.truss("trigger", "demo", "--follow")
	s.ok("trigger", "ok", "--follow")
	s.ok("trigger", "ok", "--follow")

	if out := s.ok("runs", "--status", "failed"); !strings.Contains(out, "demo") || strings.Contains(out, "succeeded") {
		t.Fatalf("failed runs =\n%s", out)
	}
	if out := s.ok("runs", "--status", "queued"); out != "no runs match\n" {
		t.Fatalf("queued runs = %q", out)
	}
	if out := s.ok("runs", "--active"); out != "no runs match\n" {
		t.Fatalf("active runs = %q", out)
	}
	if n := len(decode[[]api.Run](t, s.ok("runs", "--limit", "2", "--json"))); n != 2 {
		t.Fatalf("limit 2 gave %d runs", n)
	}
	if n := len(decode[[]api.Run](t, s.ok("runs", "ok", "--status", "succeeded", "--limit", "1", "--json"))); n != 1 {
		t.Fatalf("limit 1 gave %d runs", n)
	}
	if out := s.ok("runs", "nothing", "--json"); strings.TrimSpace(out) != "[]" {
		t.Fatalf("empty json = %q", out)
	}

	last := s.ok("runs", "show", "last")
	for _, want := range []string{"Pipeline   ok", "Status     succeeded", "Steps      1 (1 succeeded)", "hello", "#"} {
		if !strings.Contains(last, want) {
			t.Errorf("runs show last lacks %q:\n%s", want, last)
		}
	}
	failedID := runID.FindString(s.ok("runs", "demo"))
	failed := s.ok("runs", "show", failedID)
	for _, want := range []string{"Status     failed: demo failed at test", "Steps      3 (2 succeeded, 1 failed)", "failed     3     0s    2"} {
		if !strings.Contains(failed, want) {
			t.Errorf("runs show lacks %q:\n%s", want, failed)
		}
	}
	if r := decode[api.Run](t, s.ok("runs", "show", "last", "--pipeline", "demo", "--json")); r.ID != failedID {
		t.Fatalf("last of demo = %s, want %s", r.ID, failedID)
	}
}

func TestRuns_BadInputIsExplained(t *testing.T) {
	s := projectSession(t, true)
	cases := map[string][]string{
		"--status must be one of":     {"runs", "--status", "bogus"},
		"--limit must be a whole":     {"runs", "--limit", "0"},
		"cannot be used together":     {"runs", "--active", "--status", "failed"},
		"no run \"nope\"":             {"runs", "show", "nope"},
		"there are no runs yet":       {"runs", "show", "last"},
		"only goes with the id":       {"runs", "show", "abc", "--pipeline", "x"},
		"--limit must be a whole num": {"runs", "--limit", "x"},
	}
	for want, args := range cases {
		if code, _, errOut := s.truss(args...); code != 1 || !strings.Contains(errOut, want) {
			t.Errorf("truss %v: %d %q, want %q", args, code, errOut, want)
		}
	}
}

// A pipeline may be called "show": `runs show` alone still lists its runs.
func TestRuns_APipelineNamedShowStillWorks(t *testing.T) {
	s := newSession(t, true)
	s.write("show.yaml", "version: \"1\"\npipeline: {name: show}\nsteps: [{id: a, run: echo}]\n")
	s.ok("register", "show.yaml")
	s.ok("trigger", "show", "--follow")
	if out := s.ok("runs", "show"); !strings.Contains(out, "show      succeeded") {
		t.Fatalf("runs show = %q", out)
	}
}

func TestPipelines_ShowAndJSON(t *testing.T) {
	s := projectSession(t, true)
	s.ok("register", "--all")
	s.truss("trigger", "demo", "--follow")
	out := s.ok("pipelines", "show", "demo")
	for _, want := range []string{"Pipeline      demo", "Project       demo", "Concurrency   2", "0 9 * * 1-5 (UTC, missed: skip), next ", "STEPS", "test  <- lint, build", "LATEST RUNS", "failed"} {
		if !strings.Contains(out, want) {
			t.Errorf("pipelines show lacks %q:\n%s", want, out)
		}
	}
	if p := decode[api.Pipeline](t, s.ok("pipelines", "show", "demo", "--json")); p.Name != "demo" || len(p.Steps) != 3 {
		t.Fatalf("pipeline = %+v", p)
	}
	if l := decode[[]api.Pipeline](t, s.ok("pipelines", "--json")); len(l) != 2 {
		t.Fatalf("pipelines = %+v", l)
	}
	if s.ok("pipelines", "list") != s.ok("pipelines") {
		t.Error("`pipelines list` differs from `pipelines`")
	}
	if code, _, errOut := s.truss("pipelines", "show", "nope"); code != 1 || !strings.Contains(errOut, `no pipeline named "nope"`) {
		t.Fatalf("unknown: %d %q", code, errOut)
	}
}

func TestDashboard_OnceWithAndWithoutADaemon(t *testing.T) {
	s := projectSession(t, false)
	out := s.ok("dashboard", "--once")
	if !strings.Contains(out, "Daemon   not running") || !strings.Contains(out, "Flows in this project (2)") {
		t.Fatalf("dashboard =\n%s", out)
	}
	live := projectSession(t, true)
	live.ok("register", "--all")
	live.ok("trigger", "ok", "--follow")
	out = live.ok("dashboard")
	for _, want := range []string{"Daemon   running  pid ", "slots", "Flows (2)", "Latest runs", "ok  ", "ready"} {
		if !strings.Contains(out, want) {
			t.Errorf("dashboard lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "\x1b") {
		t.Errorf("a pipe gets cursor commands:\n%q", out)
	}
	for _, l := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if len([]rune(l)) != defaultWidth {
			t.Errorf("line is %d wide, want %d: %q", len([]rune(l)), defaultWidth, l)
		}
	}
}

func TestEnv_ShowsWhereThingsLive(t *testing.T) {
	s := projectSession(t, false)
	out := s.ok("env")
	for _, want := range []string{"Home          ", "(from TRUSS_HOME)", "Registry", "(none yet)", "Project       demo", "Daemon        not running"} {
		if !strings.Contains(out, want) {
			t.Errorf("env lacks %q:\n%s", want, out)
		}
	}
	if code, _, errOut := s.truss("env", "x"); code != 1 || errOut != usageText {
		t.Fatalf("env x: %d %q", code, errOut)
	}
}

func TestDaemonStatus_JSON(t *testing.T) {
	s := newSession(t, false)
	code, out, _ := s.truss("daemon", "status", "--json")
	if d := decode[daemonStatusDoc](t, out); code != 1 || d.Running || d.Daemon != nil {
		t.Fatalf("stopped: %d %+v", code, d)
	}
	up := newSession(t, true)
	if d := decode[daemonStatusDoc](t, up.ok("daemon", "status", "--json")); !d.Running || d.Daemon.PID != os.Getpid() {
		t.Fatalf("running: %+v", d)
	}
}

func TestDaemonLogs(t *testing.T) {
	s := newSession(t, false)
	if out := s.ok("daemon", "logs"); !strings.Contains(out, "no daemon log yet") {
		t.Fatalf("no log = %q", out)
	}
	if code, _, errOut := s.truss("daemon", "logs", "--follow"); code != 1 || !strings.Contains(errOut, "no daemon log yet") {
		t.Fatalf("follow, no log: %d %q", code, errOut)
	}
	logPath := filepath.Join(s.home, "daemon.log")
	if err := os.WriteFile(logPath, []byte("one\ntwo\nthree\nfour\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if out := s.ok("daemon", "logs", "-n", "2"); out != "three\nfour\n" {
		t.Fatalf("tail = %q", out)
	}
	if out := s.ok("daemon", "logs", "--lines=10"); out != "one\ntwo\nthree\nfour\n" {
		t.Fatalf("whole = %q", out)
	}
	if code, _, errOut := s.truss("daemon", "logs", "-n", "0"); code != 1 || !strings.Contains(errOut, "--lines must be") {
		t.Fatalf("-n 0: %d %q", code, errOut)
	}
	if code, _, errOut := s.truss("daemon", "logs", "extra"); code != 1 || errOut != usageText {
		t.Fatalf("extra: %d %q", code, errOut)
	}
}

func TestDaemonLogs_FollowPrintsWhatIsAppended(t *testing.T) {
	s := newSession(t, false)
	path := filepath.Join(s.home, "daemon.log")
	if err := os.WriteFile(path, []byte("first\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := &lockedBuffer{}
	done := make(chan int, 1)
	go func() { done <- Run(ctx, []string{"daemon", "logs", "-f"}, s.dir, s.env, out, &lockedBuffer{}) }()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.WriteString("second\n"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(out.String(), "second") {
		if time.Now().After(deadline) {
			t.Fatalf("never saw the appended line: %q", out.String())
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	if code := <-done; code != 0 {
		t.Fatalf("exit %d", code)
	}
	if got := out.String(); got != "first\nsecond\n" {
		t.Fatalf("followed = %q", got)
	}
}

func TestHelp_Topics(t *testing.T) {
	s := newSession(t, false)
	for topic := range helpTopics {
		out := s.ok("help", topic)
		if !strings.HasPrefix(out, "truss ") || !strings.Contains(strings.SplitN(out, "\n", 2)[0], topic) {
			t.Errorf("help %s = %q", topic, out)
		}
	}
	if s.ok("help", "flows") != s.ok("flows", "--help") || s.ok("runs", "-h") != s.ok("help", "runs") {
		t.Error("`<command> --help` differs from `help <command>`")
	}
	if s.ok("help", "pause") != helpTopics["register"] || s.ok("pause", "--help") != helpTopics["register"] {
		t.Error("pause has no help of its own and must show register's")
	}
	for _, args := range [][]string{{"help", "nope"}, {"help", "flows", "show"}} {
		if code, out, errOut := s.truss(args...); code != 1 || out != "" || errOut != usageText {
			t.Fatalf("truss %v: %d %q %q", args, code, out, errOut)
		}
	}
	if out := s.ok("help"); out != usageText {
		t.Error("bare help changed")
	}
}

// `run --help` is a path: a file may be named like a flag (spec 001).
func TestHelp_RunKeepsTreatingFlagsAsFiles(t *testing.T) {
	s := newSession(t, false)
	code, out, errOut := s.truss("run", "--help")
	if code != 1 || out != "" || strings.Contains(errOut, "truss run <name|path> [--track]\n\nRun") {
		t.Fatalf("run --help: %d %q %q", code, out, errOut)
	}
}

func TestUsage_NamesTheNewCommands(t *testing.T) {
	for _, cmd := range []string{"status", "dashboard", "project", "projects", "flows show", "flows graph", "pipelines show", "schedules", "runs show", "env", "daemon logs", "status [--json]", "help [<command>]"} {
		if !strings.Contains(usageText, cmd) {
			t.Errorf("usage does not mention %q", cmd)
		}
	}
}

// lockedBuffer is a buffer safe for one writer and one reader at once.
type lockedBuffer struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestWriteRun_ListsReasonsAndErrorsAsNotes(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 10, 0, time.UTC)
	start := now.Add(-8 * time.Second)
	code := 2
	run := api.Run{ID: "r1", Pipeline: "p", Trigger: "manual", Status: api.RunFailed, CreatedAt: start, StartedAt: &start, FinishedAt: &now,
		Steps: []api.Step{
			{ID: "a", Status: api.StepFailed, ExitCode: &code, Error: "exit 2\nsecond line", StartedAt: &start},
			{ID: "b", Status: api.StepSkipped, Reason: "step a did not succeed"},
		}}
	var b strings.Builder
	writeRun(&b, run, now, defaultWidth)
	out := b.String()
	for _, want := range []string{"Steps      2 (1 failed, 1 skipped)", "NOTES\n  a: exit 2\n  b: step a did not succeed\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "second line") {
		t.Errorf("a note runs over one line:\n%s", out)
	}
}

// A broken project file or a daemon that does not answer is part of the answer
// of `status`, not a reason to fail (spec 022 FR-001).
func TestStatus_AProblemIsReportedNotFatal(t *testing.T) {
	s := newSession(t, false)
	s.write("truss.yaml", "version: [oops\n")
	code, out, _ := s.truss("status")
	if code != 0 || !strings.Contains(out, "Project    unreadable: truss.yaml: invalid YAML") || !strings.Contains(out, "Daemon     not running") {
		t.Fatalf("status: %d\n%s", code, out)
	}
	code, raw, _ := s.truss("status", "--json")
	if doc := decode[statusDoc](t, raw); code != 0 || doc.Project != nil || !strings.Contains(doc.ProjectError, "invalid YAML") {
		t.Fatalf("status --json: %d %+v", code, doc)
	}
	// A daemon whose endpoint file cannot be read is not "not running": it is
	// a problem, and status says so and still exits 0.
	dead := newSession(t, false)
	if err := os.WriteFile(filepath.Join(dead.home, "daemon.json"), []byte("{bad"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, out, _ := dead.truss("status"); code != 0 || !strings.Contains(out, "Daemon     not answering: ") {
		t.Fatalf("unanswering daemon: %d %q", code, out)
	}
	code, raw, _ = dead.truss("status", "--json")
	if doc := decode[statusDoc](t, raw); code != 0 || doc.Daemon != nil || doc.DesktopURL != nil || doc.DaemonError == "" {
		t.Fatalf("status --json: %d %+v", code, doc)
	}
}

// Every line of a frame is as wide as the screen, with metrics, many flows and
// long names, at the smallest, the default and the largest width.
func TestRenderDashboard_LinesKeepTheScreenWidth(t *testing.T) {
	now := time.Date(2026, 11, 17, 9, 0, 0, 0, time.UTC)
	long := strings.Repeat("very-long-flow-name-", 6)
	snap := snapshot{Now: now, Status: &api.Status{PID: 1, Address: "127.0.0.1:7878", StartedAt: now.Add(-3 * time.Hour),
		RunningRuns: 9, MaxParallelRuns: 4, Home: strings.Repeat("h", 200)},
		Metrics: &api.Metrics{Supported: true, Host: &api.HostMetrics{CPUPercent: 250, MemoryTotalBytes: 16 << 30, MemoryUsedBytes: 20 << 30}}}
	for i := 0; i < 12; i++ {
		snap.Pipelines = append(snap.Pipelines, api.Pipeline{Name: long, Path: "/x", Valid: true, LastRun: &api.Run{Status: "failed", CreatedAt: now}})
		snap.Runs = append(snap.Runs, api.Run{ID: "20261117-090000-abcdef", Pipeline: long, Status: "running", CreatedAt: now, StartedAt: &now})
	}
	for _, width := range []int{minWidth, defaultWidth, maxWidth} {
		for _, name := range []string{"with a daemon", "without"} {
			s := snap
			if name == "without" {
				s = snapshot{Now: now}
			}
			frame := renderDashboard(s, []project.Flow{{Name: long}}, width)
			for _, l := range strings.Split(strings.TrimRight(frame, "\n"), "\n") {
				if n := len([]rune(l)); n != width {
					t.Errorf("%s, width %d: a line is %d wide: %q", name, width, n, l)
				}
			}
		}
	}
	if !strings.Contains(renderDashboard(snap, nil, 80), "... and 4 more") {
		t.Error("a long flow list is not cut")
	}
}

func TestCanRedraw_OnlyForATerminalFile(t *testing.T) {
	if canRedraw(&bytes.Buffer{}) {
		t.Error("a buffer is not a terminal")
	}
	f, err := os.CreateTemp(t.TempDir(), "out")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if canRedraw(f) {
		t.Error("a file is not a terminal")
	}
}

func TestSamePath(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a.yaml")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if !samePath(file, filepath.Join(dir, ".", "x", "..", "a.yaml")) {
		t.Error("two spellings of one path differ")
	}
	if samePath(file, filepath.Join(dir, "b.yaml")) {
		t.Error("two files compare equal")
	}
	link := filepath.Join(dir, "link.yaml")
	if err := os.Symlink(file, link); err == nil && !samePath(file, link) {
		t.Error("a link and its target differ")
	}
}

// The daemon log may have CRLF line ends and may be larger than what is read
// to find its end.
func TestDaemonLogs_CRLFAndLargeFiles(t *testing.T) {
	s := newSession(t, false)
	path := filepath.Join(s.home, "daemon.log")
	if err := os.WriteFile(path, []byte("one\r\ntwo\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if out := s.ok("daemon", "logs", "-n", "1"); out != "two\r\n" {
		t.Fatalf("crlf tail = %q", out)
	}
	big := strings.Repeat("0123456789abcdef\n", maxTailBytes/17+1000) + "last-1\nlast-2\n"
	if err := os.WriteFile(path, []byte(big), 0o600); err != nil {
		t.Fatal(err)
	}
	if out := s.ok("daemon", "logs", "-n", "2"); out != "last-1\nlast-2\n" {
		t.Fatalf("big tail = %q", out)
	}
	if out := s.ok("daemon", "logs", "-n", "100000"); len(out) > maxTailBytes+64 || strings.HasPrefix(out, "\n") || !strings.HasSuffix(out, "last-2\n") {
		t.Fatalf("whole big log: %d bytes", len(out))
	}
}

// validate takes a path like run does, so `validate --help` is still read as a
// file name and not as help.
func TestHelp_ValidateKeepsTreatingFlagsAsFiles(t *testing.T) {
	s := newSession(t, false)
	s.write("truss.yaml", projectFile)
	code, out, errOut := s.truss("validate", "--help")
	if code != 1 || out != "" || !strings.Contains(errOut, "--help") {
		t.Fatalf("validate --help: %d %q %q", code, out, errOut)
	}
}

// Text screens never pass a control character from a pipeline file to the
// terminal; JSON escapes it.
func TestScreens_CleanControlCharacters(t *testing.T) {
	s := newSession(t, false)
	s.write("truss.yaml", "version: \"1\"\nproject:\n  name: demo\n  description: \"nice\\e]0;pwned\\a\"\n")
	s.write(".truss/flows/evil.yaml", "version: \"1\"\npipeline: {name: \"evil\\e[2J\"}\nsteps:\n"+
		"  - {id: \"a\\e[8m\", run: \"echo safe\\r\\e[8mhidden\"}\n"+
		"  - {id: b, run: \"echo one\\necho two\\necho three\", needs: [\"a\\e[8m\"]}\n")
	for _, args := range [][]string{{"flows", "show", "evil"}, {"flows", "graph", "evil"}, {"flows"}, {"project"}, {"status"}, {"dashboard"}, {"validate", "evil"}} {
		out := s.ok(args...)
		if i := strings.IndexFunc(out, func(r rune) bool { return isControl(r) && r != '\n' }); i >= 0 {
			t.Errorf("truss %v prints a control character at %d: %q", args, i, out)
		}
	}
	if out := s.ok("flows", "show", "evil"); !strings.Contains(out, "echo one (+2 lines)") || !strings.Contains(out, "echo safe?") {
		t.Errorf("flows show =\n%s", out)
	}
	raw := s.ok("flows", "show", "evil", "--json")
	if strings.ContainsFunc(raw, func(r rune) bool { return isControl(r) && r != '\n' }) {
		t.Errorf("json has a raw control character: %q", raw)
	}
}

func TestClean(t *testing.T) {
	for in, want := range map[string]string{"plain": "plain", "a\x1b[2Jb": "a?[2Jb", "x\ry\a": "x?y?", "t\tt": "t t", "c1\u009b": "c1?", "héllo": "héllo", "a\u202eb": "a?b", "z\u200bw": "z?w"} {
		if got := clean(in); got != want {
			t.Errorf("clean(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDaemonLogs_FollowsAReplacedFile(t *testing.T) {
	s := newSession(t, false)
	path := filepath.Join(s.home, "daemon.log")
	if err := os.WriteFile(path, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := &lockedBuffer{}
	done := make(chan int, 1)
	go func() { done <- Run(ctx, []string{"daemon", "logs", "-f"}, s.dir, s.env, out, &lockedBuffer{}) }()
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(out.String(), "old") && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	// Replace the file: a new file with a longer content, so only the file
	// identity tells it from the old one.
	next := path + ".new"
	if err := os.WriteFile(next, []byte("brand new content\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(next, path); err != nil {
		t.Skipf("cannot replace an open file here: %v", err)
	}
	for !strings.Contains(out.String(), "brand new content") {
		if time.Now().After(deadline.Add(5 * time.Second)) {
			t.Fatalf("never saw the replacement: %q", out.String())
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	<-done
}

// An exact name wins over another pipeline's registered name (--name).
func TestPipelines_ShowPrefersTheExactName(t *testing.T) {
	s := newSession(t, true)
	s.write("a.yaml", "version: \"1\"\npipeline: {name: first}\nsteps: [{id: a, run: echo}]\n")
	s.write("b.yaml", "version: \"1\"\npipeline: {name: alias}\nsteps: [{id: b, run: echo}]\n")
	s.ok("register", "a.yaml", "--name", "alias2")
	s.ok("register", "b.yaml")
	s.ok("register", "a.yaml", "--name", "other") // a second registration of a.yaml under another name
	if p := decode[api.Pipeline](t, s.ok("pipelines", "show", "alias", "--json")); p.Name != "alias" || len(p.Steps) != 1 || p.Steps[0].ID != "b" {
		t.Fatalf("pipelines show alias = %+v", p)
	}
}
