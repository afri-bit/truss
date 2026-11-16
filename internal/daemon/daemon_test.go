package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/afri-bit/truss/internal/api"
	"github.com/afri-bit/truss/internal/home"
	"github.com/afri-bit/truss/internal/registry"
)

// fakeClock is a clock tests move by hand.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}

func today(hour, minute, second int) time.Time {
	return time.Date(2026, 11, 11, hour, minute, second, 0, time.Local)
}

// boot starts a daemon on a free port and returns a client for it.
func boot(t *testing.T, h home.Dir, clock *fakeClock) (*Daemon, *api.Client) {
	t.Helper()
	opts := Options{Home: h, Address: "127.0.0.1:0", Version: "test", Env: os.Environ(), Tick: 5 * time.Millisecond}
	if clock != nil {
		opts.Now = clock.Now
	}
	d, err := Start(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- d.Serve(context.Background()) }()
	t.Cleanup(func() {
		d.Stop()
		if err := <-done; err != nil {
			t.Errorf("Serve: %v", err)
		}
	})
	c, err := api.Dial(context.Background(), h)
	if err != nil {
		t.Fatal(err)
	}
	return d, c
}

func writeDefinition(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

const helloDefinition = `version: "1"
pipeline:
  name: hello
steps:
  - id: greet
    run: echo hello from the daemon
`

func scheduled(missed string) string {
	return `version: "1"
pipeline:
  name: nightly
on:
  schedule:
    - cron: "0 9 * * *"
      missed: ` + missed + `
steps:
  - id: work
    run: echo scheduled work
`
}

func awaitRuns(t *testing.T, c *api.Client, name string, want int) []api.Run {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		list, err := c.Runs(context.Background(), name, 0)
		if err != nil {
			t.Fatal(err)
		}
		settled := len(list) >= want
		for _, r := range list {
			settled = settled && !r.Active()
		}
		if settled {
			return list
		}
		if time.Now().After(deadline) {
			t.Fatalf("runs of %s = %+v, want %d finished", name, list, want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestDaemon_RegisterTriggerLogUnregister(t *testing.T) {
	ctx := context.Background()
	h := home.Dir(t.TempDir())
	_, c := boot(t, h, nil)
	path := writeDefinition(t, t.TempDir(), "truss.yaml", helloDefinition)

	p, err := c.Register(ctx, api.RegisterRequest{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "hello" || !p.Valid || len(p.Steps) != 1 || p.Overlap != "queue" || p.Concurrency != 1 {
		t.Fatalf("pipeline = %+v", p)
	}
	run, err := c.Trigger(ctx, "hello")
	if err != nil {
		t.Fatal(err)
	}
	list := awaitRuns(t, c, "hello", 1)
	if list[0].ID != run.ID || list[0].Status != api.RunSucceeded || list[0].Trigger != api.TriggerManual {
		t.Fatalf("run = %+v", list[0])
	}
	data, _, status, err := c.Log(ctx, run.ID, 0)
	if err != nil || status != api.RunSucceeded || !strings.Contains(string(data), "step greet | hello from the daemon") {
		t.Fatalf("log (%s, %v):\n%s", status, err, data)
	}
	st, err := c.Status(ctx)
	if err != nil || st.Pipelines != 1 || st.Version != "test" {
		t.Fatalf("status = %+v, %v", st, err)
	}

	if err := c.Unregister(ctx, "hello"); err != nil {
		t.Fatal(err)
	}
	if list, _ := c.Pipelines(ctx); len(list) != 0 {
		t.Fatalf("still registered: %+v", list)
	}
	var apiErr *api.StatusError
	if _, err := c.Trigger(ctx, "hello"); !errors.As(err, &apiErr) || apiErr.Code != http.StatusNotFound {
		t.Fatalf("trigger after unregister: %v", err)
	}
	if got, err := c.Run(ctx, run.ID); err != nil || got.ID != run.ID {
		t.Fatalf("history lost after unregister: %v", err)
	}
}

func TestDaemon_RejectsInvalidRegistrations(t *testing.T) {
	ctx := context.Background()
	_, c := boot(t, home.Dir(t.TempDir()), nil)
	dir := t.TempDir()
	cases := map[string]struct {
		req  api.RegisterRequest
		code int
	}{
		"relative path":  {api.RegisterRequest{Path: "truss.yaml"}, http.StatusBadRequest},
		"missing file":   {api.RegisterRequest{Path: filepath.Join(dir, "nope.yaml")}, http.StatusUnprocessableEntity},
		"invalid file":   {api.RegisterRequest{Path: writeDefinition(t, dir, "bad.yaml", "version: \"2\"\n")}, http.StatusUnprocessableEntity},
		"bad name":       {api.RegisterRequest{Path: writeDefinition(t, dir, "ok.yaml", helloDefinition), Name: "has space"}, http.StatusBadRequest},
		"unusable title": {api.RegisterRequest{Path: writeDefinition(t, dir, "t.yaml", strings.Replace(helloDefinition, "name: hello", "name: my pipeline", 1))}, http.StatusBadRequest},
	}
	for name, tc := range cases {
		var apiErr *api.StatusError
		if _, err := c.Register(ctx, tc.req); !errors.As(err, &apiErr) || apiErr.Code != tc.code {
			t.Errorf("%s: err = %v, want status %d", name, err, tc.code)
		}
	}
	// The same file under a second name is fine; another file under a taken name is not.
	ok := filepath.Join(dir, "ok.yaml")
	if _, err := c.Register(ctx, api.RegisterRequest{Path: ok}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Register(ctx, api.RegisterRequest{Path: ok, Name: "again"}); err != nil {
		t.Fatal(err)
	}
	var apiErr *api.StatusError
	if _, err := c.Register(ctx, api.RegisterRequest{Path: filepath.Join(dir, "t.yaml"), Name: "hello"}); !errors.As(err, &apiErr) || apiErr.Code != http.StatusConflict {
		t.Fatalf("taken name: %v", err)
	}
}

func TestDaemon_RequiresTheTokenAndALocalHost(t *testing.T) {
	d, _ := boot(t, home.Dir(t.TempDir()), nil)
	url := "http://" + d.Endpoint().Address + api.Prefix + "/status"
	get := func(token, host string) int {
		req, _ := http.NewRequest(http.MethodGet, url, nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if host != "" {
			req.Host = host
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		return resp.StatusCode
	}
	if code := get("", ""); code != http.StatusUnauthorized {
		t.Errorf("no token: %d", code)
	}
	if code := get("wrong", ""); code != http.StatusUnauthorized {
		t.Errorf("wrong token: %d", code)
	}
	if code := get(d.Endpoint().Token, "evil.example:7878"); code != http.StatusForbidden {
		t.Errorf("foreign host: %d", code)
	}
	if code := get(d.Endpoint().Token, ""); code != http.StatusOK {
		t.Errorf("valid request: %d", code)
	}
	resp, err := http.Get("http://" + d.Endpoint().Address + "/")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("desktop page: %v %v", resp, err)
	}
	_ = resp.Body.Close()
}

func TestDaemon_RefusesASecondInstanceAndNonLoopbackAddresses(t *testing.T) {
	h := home.Dir(t.TempDir())
	boot(t, h, nil)
	if _, err := Start(context.Background(), Options{Home: h, Address: "127.0.0.1:0"}); err == nil || !strings.Contains(err.Error(), "already running") {
		t.Fatalf("second daemon: %v", err)
	}
	if _, err := Start(context.Background(), Options{Home: home.Dir(t.TempDir()), Address: "0.0.0.0:0"}); err == nil {
		t.Fatal("public address accepted")
	}
}

func TestDaemon_ScheduleFiresWhenTheClockReachesIt(t *testing.T) {
	ctx := context.Background()
	clock := &fakeClock{now: today(8, 59, 50)}
	_, c := boot(t, home.Dir(t.TempDir()), clock)
	path := writeDefinition(t, t.TempDir(), "nightly.yaml", scheduled("skip"))
	p, err := c.Register(ctx, api.RegisterRequest{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Schedules) != 1 || p.Schedules[0].Next == nil || !p.Schedules[0].Next.Equal(today(9, 0, 0)) {
		t.Fatalf("schedules = %+v", p.Schedules)
	}
	time.Sleep(30 * time.Millisecond) // several ticks before the time: nothing may start
	if list, _ := c.Runs(ctx, "nightly", 0); len(list) != 0 {
		t.Fatalf("ran early: %+v", list)
	}

	clock.Set(today(9, 0, 2))
	list := awaitRuns(t, c, "nightly", 1)
	if list[0].Trigger != api.TriggerSchedule || list[0].Status != api.RunSucceeded || !list[0].ScheduledFor.Equal(today(9, 0, 0)) {
		t.Fatalf("run = %+v", list[0])
	}
	time.Sleep(30 * time.Millisecond)
	if list, _ := c.Runs(ctx, "nightly", 0); len(list) != 1 {
		t.Fatalf("fired more than once: %+v", list)
	}
}

func TestDaemon_PausedPipelineDoesNotFire(t *testing.T) {
	ctx := context.Background()
	clock := &fakeClock{now: today(8, 59, 50)}
	_, c := boot(t, home.Dir(t.TempDir()), clock)
	path := writeDefinition(t, t.TempDir(), "nightly.yaml", scheduled("skip"))
	if _, err := c.Register(ctx, api.RegisterRequest{Path: path}); err != nil {
		t.Fatal(err)
	}
	p, err := c.SetPaused(ctx, "nightly", true)
	if err != nil || !p.Paused || p.Schedules[0].Next != nil {
		t.Fatalf("paused = %+v, %v", p, err)
	}
	clock.Set(today(9, 0, 2))
	time.Sleep(40 * time.Millisecond)
	if list, _ := c.Runs(ctx, "nightly", 0); len(list) != 0 {
		t.Fatalf("paused pipeline ran: %+v", list)
	}
	// Resumed at 09:05: the 09:00 time is not made up for, the next one is tomorrow.
	clock.Set(today(9, 5, 0))
	p, _ = c.SetPaused(ctx, "nightly", false)
	if p.Schedules[0].Next == nil || !p.Schedules[0].Next.Equal(today(9, 0, 0).AddDate(0, 0, 1)) {
		t.Fatalf("next after resume = %+v", p.Schedules[0])
	}
}

// seed prepares a home as a daemon left it at 08:00 with one scheduled pipeline.
func seed(t *testing.T, missed string) home.Dir {
	t.Helper()
	h := home.Dir(t.TempDir())
	if err := h.Ensure(); err != nil {
		t.Fatal(err)
	}
	path := writeDefinition(t, t.TempDir(), "nightly.yaml", scheduled(missed))
	reg, err := registry.Open(h.Registry())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reg.Add("nightly", path, today(7, 0, 0)); err != nil {
		t.Fatal(err)
	}
	if err := home.WriteJSON(h.Clock(), clockFile{LastSeen: today(8, 0, 0)}); err != nil {
		t.Fatal(err)
	}
	return h
}

// The laptop was off from 08:00 to 11:00. The 09:00 run must not start late.
func TestDaemon_ScheduleMissedWhileOffIsSkippedAndRecorded(t *testing.T) {
	h := seed(t, "skip")
	_, c := boot(t, h, &fakeClock{now: today(11, 0, 0)})
	awaitRuns(t, c, "nightly", 1)
	time.Sleep(30 * time.Millisecond)
	list, _ := c.Runs(context.Background(), "nightly", 0)
	if len(list) != 1 {
		t.Fatalf("runs = %+v", list)
	}
	r := list[0]
	if r.Status != api.RunSkipped || r.StartedAt != nil || !r.ScheduledFor.Equal(today(9, 0, 0)) || !strings.HasPrefix(r.Reason, "missed") {
		t.Fatalf("run = %+v", r)
	}
}

func TestDaemon_ScheduleMissedWhileOffCatchesUpWhenAsked(t *testing.T) {
	h := seed(t, "run")
	_, c := boot(t, h, &fakeClock{now: today(11, 0, 0)})
	list := awaitRuns(t, c, "nightly", 1)
	if len(list) != 1 || list[0].Trigger != api.TriggerCatchUp || list[0].Status != api.RunSucceeded {
		t.Fatalf("runs = %+v", list)
	}
}

func TestDaemon_SeesEditsToARegisteredFile(t *testing.T) {
	ctx := context.Background()
	_, c := boot(t, home.Dir(t.TempDir()), nil)
	path := writeDefinition(t, t.TempDir(), "truss.yaml", helloDefinition)
	if _, err := c.Register(ctx, api.RegisterRequest{Path: path}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("version: \"1\"\npipeline: {name: hello}\nsteps: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var apiErr *api.StatusError
	if _, err := c.Trigger(ctx, "hello"); !errors.As(err, &apiErr) || apiErr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("trigger of a broken file: %v", err)
	}
	list, _ := c.Pipelines(ctx)
	if len(list) != 1 || list[0].Valid || list[0].Error == "" {
		t.Fatalf("pipeline = %+v", list)
	}
}

func TestDaemon_RemovesItsEndpointFileOnStop(t *testing.T) {
	h := home.Dir(t.TempDir())
	d, err := Start(context.Background(), Options{Home: h, Address: "127.0.0.1:0"})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- d.Serve(context.Background()) }()
	c, err := api.Dial(context.Background(), h)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(h.Endpoint()); !os.IsNotExist(err) {
		t.Fatalf("endpoint file left behind: %v", err)
	}
	if _, err := api.Dial(context.Background(), h); !errors.Is(err, api.ErrNotRunning) {
		t.Fatalf("dial a stopped daemon: %v", err)
	}
}

const retryConditionDefinition = `version: "1"
pipeline:
  name: rich
steps:
  - id: lint
    run: exit 1
    retries: 1
    continue-on-error: true
  - id: report
    run: echo reported
    needs: [lint]
    if: failure()
  - id: publish
    run: echo published
    needs: [lint]
  - id: dev-only
    run: echo dev
    if: env.TRUSS_TRIGGER == 'schedule'
`

func TestDaemon_RunShowsAttemptsContinuedAndSkippedSteps(t *testing.T) {
	ctx := context.Background()
	_, c := boot(t, home.Dir(t.TempDir()), nil)
	path := writeDefinition(t, t.TempDir(), "truss.yaml", retryConditionDefinition)
	if _, err := c.Register(ctx, api.RegisterRequest{Path: path}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Trigger(ctx, "rich"); err != nil {
		t.Fatal(err)
	}
	run := awaitRuns(t, c, "rich", 1)[0]
	if run.Status != api.RunSucceeded {
		t.Fatalf("run = %+v", run)
	}
	got := map[string]api.Step{}
	for _, s := range run.Steps {
		got[s.ID] = s
	}
	if s := got["lint"]; s.Status != api.StepFailed || s.Attempts != 2 || !s.ContinuedOnError {
		t.Fatalf("lint = %+v", s)
	}
	if s := got["report"]; s.Status != api.StepSucceeded {
		t.Fatalf("report = %+v", s)
	}
	if s := got["publish"]; s.Status != api.StepSkipped || s.Reason != "step lint did not succeed" {
		t.Fatalf("publish = %+v", s)
	}
	if s := got["dev-only"]; s.Status != api.StepSkipped || s.Reason != "its if condition is false" {
		t.Fatalf("dev-only = %+v", s)
	}

	// The wire format: new fields appear only where they apply.
	raw, _ := json.Marshal(run.Steps)
	text := string(raw)
	for _, want := range []string{`"attempts":2`, `"continued_on_error":true`, `"reason":"step lint did not succeed"`} {
		if !strings.Contains(text, want) {
			t.Errorf("steps JSON lacks %s: %s", want, text)
		}
	}
	if strings.Count(text, `"attempts"`) != 1 || strings.Count(text, `"continued_on_error"`) != 1 {
		t.Errorf("new fields leaked onto other steps: %s", text)
	}
	data, _, _, err := c.Log(ctx, run.ID, 0)
	if err != nil || !strings.Contains(string(data), "step lint retry 2 of 2 in 0.000s") {
		t.Fatalf("log (%v):\n%s", err, data)
	}
}

func TestDaemon_RejectsAnInvalidCondition(t *testing.T) {
	_, c := boot(t, home.Dir(t.TempDir()), nil)
	path := writeDefinition(t, t.TempDir(), "truss.yaml", "version: \"1\"\npipeline:\n  name: bad\nsteps:\n  - id: a\n    run: echo a\n    if: sometimes()\n")
	_, err := c.Register(context.Background(), api.RegisterRequest{Path: path})
	var apiErr *api.StatusError
	if !errors.As(err, &apiErr) || apiErr.Code != http.StatusUnprocessableEntity || !strings.Contains(err.Error(), "unknown function") {
		t.Fatalf("Register error = %v", err)
	}
}

func TestCheckAddress(t *testing.T) {
	for addr, ok := range map[string]bool{
		"127.0.0.1:7878": true, "127.0.0.1:0": true, "[::1]:9000": true,
		"0.0.0.0:7878": false, "localhost": false, "127.0.0.1:abc": false,
		"127.0.0.1:70000": false, `127.0.0.1:1" --max-runs 5 "`: false,
	} {
		if err := CheckAddress(addr); (err == nil) != ok {
			t.Errorf("CheckAddress(%q) = %v, want ok=%v", addr, err, ok)
		}
	}
}

// queuedRecord writes the run.json a daemon leaves behind for a run that was
// queued when it stopped.
func queuedRecord(t *testing.T, h home.Dir, id, pipelineName string) {
	t.Helper()
	dir := filepath.Join(h.Runs(), id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	run := api.Run{ID: id, Pipeline: pipelineName, Trigger: api.TriggerManual, Status: api.RunQueued,
		CreatedAt: time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC), Steps: []api.Step{{ID: "greet", Status: api.StepPending}}}
	if err := home.WriteJSON(filepath.Join(dir, "run.json"), run); err != nil {
		t.Fatal(err)
	}
}

func TestDaemon_StartsRunsThatWereQueuedWhenItStopped(t *testing.T) {
	h := home.Dir(t.TempDir())
	dir := t.TempDir()
	path := writeDefinition(t, dir, "hello.yaml", helloDefinition)

	_, c := boot(t, h, nil)
	if _, err := c.Register(context.Background(), api.RegisterRequest{Path: path}); err != nil {
		t.Fatal(err)
	}
	if err := c.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitStopped(t, h)

	queuedRecord(t, h, "20261111-090000-aaaaaa", "hello")
	queuedRecord(t, h, "20261111-090001-bbbbbb", "hello")
	queuedRecord(t, h, "20261111-090002-cccccc", "gone")

	_, c = boot(t, h, nil)
	list := awaitRuns(t, c, "hello", 2)
	for _, r := range list {
		if r.Status != api.RunSucceeded || r.Trigger != api.TriggerManual {
			t.Fatalf("restored run = %+v", r)
		}
	}
	if list[0].StartedAt.Before(*list[1].StartedAt) {
		t.Fatalf("runs started out of request order: %+v", list)
	}
	log, _, _, err := c.Log(context.Background(), "20261111-090000-aaaaaa", 0)
	if err != nil || !strings.Contains(string(log), "step greet | hello from the daemon") {
		t.Fatalf("log = %q, %v", log, err)
	}
	gone, err := c.Run(context.Background(), "20261111-090002-cccccc")
	if err != nil || gone.Status != api.RunFailed || !strings.Contains(gone.Reason, "could not be restored") {
		t.Fatalf("run of an unregistered pipeline = %+v, %v", gone, err)
	}
}

// waitStopped waits until the daemon of h has removed its endpoint file.
func waitStopped(t *testing.T, h home.Dir) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(h.Endpoint()); os.IsNotExist(err) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the daemon did not stop")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestDaemon_RunsStartInWorkdirAndSeeTheDataFolders(t *testing.T) {
	ctx := context.Background()
	h := home.Dir(t.TempDir())
	_, c := boot(t, h, nil)
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "work"), 0o750); err != nil {
		t.Fatal(err)
	}
	show := "echo $TRUSS_DATA_DIR && echo $TRUSS_CACHE_DIR && pwd"
	if runtime.GOOS == "windows" {
		show = "echo %TRUSS_DATA_DIR% && echo %TRUSS_CACHE_DIR% && cd"
	}
	path := writeDefinition(t, dir, "truss.yaml", "version: \"1\"\npipeline:\n  name: spaces\n  workdir: work\nsteps:\n  - id: show\n    run: "+show+"\n")
	if _, err := c.Register(ctx, api.RegisterRequest{Path: path}); err != nil {
		t.Fatal(err)
	}
	run, err := c.Trigger(ctx, "spaces")
	if err != nil {
		t.Fatal(err)
	}
	awaitRuns(t, c, "spaces", 1)
	data, _, status, err := c.Log(ctx, run.ID, 0)
	if err != nil || status != api.RunSucceeded {
		t.Fatalf("log (%s, %v):\n%s", status, err, data)
	}
	var lines []string
	for _, l := range strings.Split(string(data), "\n") {
		if _, v, ok := strings.Cut(l, " step show | "); ok {
			lines = append(lines, strings.TrimSpace(v))
		}
	}
	// The shell may print a symlink-resolved or short-named path, so the
	// working directory is checked by its last element only.
	if len(lines) != 3 || lines[0] != h.PipelineData("spaces") || lines[1] != h.PipelineCache("spaces") || filepath.Base(lines[2]) != "work" {
		t.Errorf("step output = %q\nlog:\n%s", lines, data)
	}
	if info, err := os.Stat(h.PipelineCache("spaces")); err != nil || !info.IsDir() {
		t.Fatalf("cache folder: %v", err)
	}

	// Unregistering keeps the data folder.
	marker := filepath.Join(h.PipelineCache("spaces"), "keep")
	if err := os.WriteFile(marker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := c.Unregister(ctx, "spaces"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("data folder lost after unregister: %v", err)
	}
}

func TestDaemon_MissingWorkdirFailsTheRunWithoutAnExitCode(t *testing.T) {
	ctx := context.Background()
	_, c := boot(t, home.Dir(t.TempDir()), nil)
	path := writeDefinition(t, t.TempDir(), "truss.yaml", "version: \"1\"\npipeline:\n  name: gone\n  workdir: nope\nsteps:\n  - id: s\n    run: echo hi\n")
	if _, err := c.Register(ctx, api.RegisterRequest{Path: path}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Trigger(ctx, "gone"); err != nil {
		t.Fatal(err)
	}
	list := awaitRuns(t, c, "gone", 1)
	if list[0].Status != api.RunFailed || len(list[0].Steps) == 0 || list[0].Steps[0].ExitCode != nil {
		t.Fatalf("run = %+v", list[0])
	}
}

func TestDaemon_RestoredQueuedRunSeesTheDataFolders(t *testing.T) {
	h := home.Dir(t.TempDir())
	show := "echo $TRUSS_CACHE_DIR"
	if runtime.GOOS == "windows" {
		show = "echo %TRUSS_CACHE_DIR%"
	}
	path := writeDefinition(t, t.TempDir(), "truss.yaml", "version: \"1\"\npipeline:\n  name: restored\nsteps:\n  - id: show\n    run: "+show+"\n")
	_, c := boot(t, h, nil)
	if _, err := c.Register(context.Background(), api.RegisterRequest{Path: path}); err != nil {
		t.Fatal(err)
	}
	if err := c.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitStopped(t, h)
	queuedRecord(t, h, "20261111-090000-aaaaaa", "restored")

	_, c = boot(t, h, nil)
	awaitRuns(t, c, "restored", 1)
	log, _, _, err := c.Log(context.Background(), "20261111-090000-aaaaaa", 0)
	if err != nil || !strings.Contains(string(log), "step show | "+h.PipelineCache("restored")) {
		t.Fatalf("log = %q, %v", log, err)
	}
}

// FR-017: a run restored after a restart runs in the project root, like a
// run submitted to the running daemon.
func TestDaemon_RestoredQueuedRunOfAFlowRunsInTheProjectRoot(t *testing.T) {
	h := home.Dir(t.TempDir())
	pwd := "pwd"
	if runtime.GOOS == "windows" {
		pwd = "cd"
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	writeDefinition(t, root, "truss.yaml", "version: \"1\"\nproject:\n  name: demo\n")
	if err := os.MkdirAll(filepath.Join(root, ".truss", "flows"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := writeDefinition(t, filepath.Join(root, ".truss", "flows"), "where.yaml", "version: \"1\"\npipeline:\n  name: where\nsteps:\n  - id: here\n    run: "+pwd+"\n")
	_, c := boot(t, h, nil)
	if _, err := c.Register(context.Background(), api.RegisterRequest{Path: path}); err != nil {
		t.Fatal(err)
	}
	if err := c.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitStopped(t, h)
	queuedRecord(t, h, "20261111-090000-aaaaaa", "where")

	_, c = boot(t, h, nil)
	awaitRuns(t, c, "where", 1)
	log, _, _, err := c.Log(context.Background(), "20261111-090000-aaaaaa", 0)
	// Compare the last element only: a Windows 8.3 temp path prints differently.
	// A run in <root>/.truss/flows would end in "flows" instead.
	var printed string
	for _, line := range strings.Split(string(log), "\n") {
		if _, p, ok := strings.Cut(strings.TrimRight(line, "\r"), "step here | "); ok {
			printed = p
		}
	}
	if err != nil || printed == "" || filepath.Base(printed) != filepath.Base(root) {
		t.Fatalf("log = %q, %v; want the step to run in %s", log, err, root)
	}
}

func TestDaemon_MetricsReportTheHostAndARunningStep(t *testing.T) {
	h := home.Dir(t.TempDir())
	_, c := boot(t, h, nil)
	ctx := context.Background()

	m, err := c.Metrics(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if m.OS != runtime.GOOS || m.Daemon.PID != os.Getpid() || m.Steps == nil || m.History == nil {
		t.Fatalf("metrics = %+v", m)
	}
	measured := map[string]bool{"linux": true, "darwin": true, "windows": true}
	if !measured[runtime.GOOS] {
		if m.Supported || m.Reason == "" {
			t.Fatalf("an unsupported OS must say why: %+v", m)
		}
		return
	}

	long := "sleep 30"
	if runtime.GOOS == "windows" {
		long = "ping -n 31 127.0.0.1 >NUL"
	}
	dir := t.TempDir()
	path := writeDefinition(t, dir, "truss.yaml", "version: \"1\"\npipeline:\n  name: busy\nsteps:\n  - id: wait\n    run: "+long+"\n")
	if _, err := c.Register(ctx, api.RegisterRequest{Path: path}); err != nil {
		t.Fatal(err)
	}
	run, err := c.Trigger(ctx, "busy")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		// Wait for the step to be gone: on Windows its shell would still hold dir open.
		_, _ = c.Cancel(ctx, run.ID)
		for end := time.Now().Add(20 * time.Second); time.Now().Before(end); time.Sleep(50 * time.Millisecond) {
			if r, err := c.Run(ctx, run.ID); err != nil || !r.Active() {
				return
			}
		}
	}()

	deadline := time.Now().Add(20 * time.Second)
	for {
		got, err := c.Run(ctx, run.ID)
		if err != nil {
			t.Fatal(err)
		}
		m, err = c.Metrics(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if pid := got.Steps[0].PID; pid > 0 && len(m.Steps) == 1 && m.Steps[0].PID == pid && m.Host != nil && len(m.History) > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("run %+v\nmetrics %+v", got, m)
		}
		time.Sleep(100 * time.Millisecond)
	}
	st := m.Steps[0]
	if st.RunID != run.ID || st.StepID != "wait" || st.RSSBytes == 0 || st.Processes < 1 {
		t.Errorf("step metrics = %+v", st)
	}
	if runtime.GOOS != "windows" && !st.Tree {
		t.Errorf("on %s a step's whole process group is measured", runtime.GOOS)
	}
	if m.Host.MemoryTotalBytes == 0 || len(m.Host.Cores) == 0 || m.Daemon.RSSBytes == 0 {
		t.Errorf("host %+v daemon %+v", m.Host, m.Daemon)
	}
}

// pwdLine runs the registered pipeline name once and returns the directory its
// step printed.
func pwdLine(t *testing.T, c *api.Client, name string) string {
	t.Helper()
	ctx := context.Background()
	run, err := c.Trigger(ctx, name)
	if err != nil {
		t.Fatal(err)
	}
	awaitRuns(t, c, name, 1)
	data, _, status, err := c.Log(ctx, run.ID, 0)
	if err != nil || status != api.RunSucceeded {
		t.Fatalf("log (%s, %v):\n%s", status, err, data)
	}
	for _, l := range strings.Split(string(data), "\n") {
		if _, v, ok := strings.Cut(l, " step where | "); ok {
			return strings.TrimSpace(v)
		}
	}
	t.Fatalf("no output of the step:\n%s", data)
	return ""
}

func pwdDefinition(name string) string {
	where := "pwd"
	if runtime.GOOS == "windows" {
		where = "cd"
	}
	head := "version: \"1\"\n"
	if name != "" {
		head += "pipeline:\n  name: " + name + "\n"
	}
	return head + "steps:\n  - id: where\n    run: " + where + "\n"
}

// Spec 016 FR-017: a flow runs in the project root; any other file in its folder.
func TestDaemon_FlowRunsInTheProjectRoot(t *testing.T) {
	ctx := context.Background()
	_, c := boot(t, home.Dir(t.TempDir()), nil)
	root := filepath.Join(t.TempDir(), "myproject")
	writeDefinition(t, mkdirs(t, root), "truss.yaml", "version: \"1\"\nproject:\n  name: myproject\n")
	flow := writeDefinition(t, mkdirs(t, filepath.Join(root, ".truss", "flows")), "where.yaml", pwdDefinition("inroot"))
	if _, err := c.Register(ctx, api.RegisterRequest{Path: flow}); err != nil {
		t.Fatal(err)
	}
	// The shell may print a resolved or short-named path: compare the last element.
	if got := pwdLine(t, c, "inroot"); filepath.Base(got) != "myproject" {
		t.Fatalf("step ran in %q, want the project root", got)
	}

	other := filepath.Join(t.TempDir(), "elsewhere")
	file := writeDefinition(t, mkdirs(t, other), "where.yaml", pwdDefinition("plain"))
	if _, err := c.Register(ctx, api.RegisterRequest{Path: file}); err != nil {
		t.Fatal(err)
	}
	if got := pwdLine(t, c, "plain"); filepath.Base(got) != "elsewhere" {
		t.Fatalf("step ran in %q, want the file's folder", got)
	}
}

func TestDaemon_FlowWithoutProjectFileRunsInItsFolder(t *testing.T) {
	ctx := context.Background()
	_, c := boot(t, home.Dir(t.TempDir()), nil)
	root := filepath.Join(t.TempDir(), "noproject")
	flows := mkdirs(t, filepath.Join(root, ".truss", "flows"))
	flow := writeDefinition(t, flows, "where.yaml", pwdDefinition("noroot"))
	if _, err := c.Register(ctx, api.RegisterRequest{Path: flow}); err != nil {
		t.Fatal(err)
	}
	if got := pwdLine(t, c, "noroot"); filepath.Base(got) != "flows" {
		t.Fatalf("step ran in %q, want .truss/flows", got)
	}
}

// Spec 016 FR-008: a pipeline without a name registers under its file stem.
func TestDaemon_UnnamedPipelineUsesTheFileStem(t *testing.T) {
	ctx := context.Background()
	_, c := boot(t, home.Dir(t.TempDir()), nil)
	path := writeDefinition(t, t.TempDir(), "stemmed.yaml", pwdDefinition(""))
	p, err := c.Register(ctx, api.RegisterRequest{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "stemmed" || !p.Valid {
		t.Fatalf("pipeline = %+v", p)
	}
}

func mkdirs(t *testing.T, dir string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	return dir
}
