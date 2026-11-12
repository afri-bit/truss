package daemon

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/afri-bit/truss/internal/api"
	"github.com/afri-bit/truss/internal/home"
)

// call sends one raw request to a daemon and returns the status code and body.
func call(t *testing.T, d *Daemon, token, method, path string, body []byte) (int, string) {
	t.Helper()
	req, err := http.NewRequest(method, "http://"+d.Endpoint().Address+api.Prefix+path, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var out bytes.Buffer
	_, _ = out.ReadFrom(resp.Body)
	return resp.StatusCode, out.String()
}

func TestTrackingAPI_RecordsALocalRunEndToEnd(t *testing.T) {
	ctx := context.Background()
	_, c := boot(t, home.Dir(t.TempDir()), nil)

	run, err := c.CreateRun(ctx, api.CreateRunRequest{Pipeline: "local-ci", Steps: []api.PlanStep{{ID: "build"}, {ID: "test", Needs: []string{"build"}}}})
	if err != nil || run.Status != api.RunRunning || run.Trigger != api.TriggerLocal || len(run.Steps) != 2 || run.Steps[1].Needs[0] != "build" {
		t.Fatalf("create = %+v, %v", run, err)
	}
	if _, err := c.ReportStep(ctx, run.ID, api.StepUpdate{ID: "build", Status: api.StepRunning}); err != nil {
		t.Fatal(err)
	}
	if st, _ := c.Status(ctx); st.RunningRuns != 1 {
		t.Fatalf("a running local run is not counted: %+v", st)
	}
	next, status, err := c.AppendLog(ctx, run.ID, 0, []byte("hello\n"))
	if err != nil || next != 6 || status != api.RunRunning {
		t.Fatalf("append: %d %s %v", next, status, err)
	}
	if _, err := c.ReportStep(ctx, run.ID, api.StepUpdate{ID: "build", Status: api.StepSucceeded, ExitCode: new(int)}); err != nil {
		t.Fatal(err)
	}
	done, err := c.FinishRun(ctx, run.ID, api.FinishRequest{Status: api.RunSucceeded})
	if err != nil || done.Status != api.RunSucceeded {
		t.Fatalf("finish = %+v, %v", done, err)
	}

	data, off, status, err := c.Log(ctx, run.ID, 0)
	if err != nil || string(data) != "hello\n" || off != 6 || status != api.RunSucceeded {
		t.Fatalf("log = %q %d %s %v", data, off, status, err)
	}
	list, err := c.Runs(ctx, "local-ci", 0)
	if err != nil || len(list) != 1 || list[0].ID != run.ID || list[0].Trigger != api.TriggerLocal {
		t.Fatalf("runs = %+v, %v", list, err)
	}
	if st, _ := c.Status(ctx); st.RunningRuns != 0 {
		t.Fatalf("status = %+v", st)
	}
}

func TestTrackingAPI_RequiresTheToken(t *testing.T) {
	d, c := boot(t, home.Dir(t.TempDir()), nil)
	run, err := c.CreateRun(context.Background(), api.CreateRunRequest{Pipeline: "p", Steps: []api.PlanStep{{ID: "a"}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range []struct{ method, path, body string }{
		{"POST", "/runs", `{"pipeline":"p","steps":[{"id":"a"}]}`},
		{"POST", "/runs/" + run.ID + "/steps", `{"id":"a","status":"running"}`},
		{"POST", "/runs/" + run.ID + "/log?offset=0", "forged"},
		{"POST", "/runs/" + run.ID + "/finish", `{"status":"succeeded"}`},
	} {
		for _, token := range []string{"", "wrong"} {
			if code, _ := call(t, d, token, e.method, e.path, []byte(e.body)); code != http.StatusUnauthorized {
				t.Errorf("%s %s with token %q: %d", e.method, e.path, token, code)
			}
		}
	}
	after, _ := c.Run(context.Background(), run.ID)
	if after.Status != api.RunRunning || after.Steps[0].Status != api.StepPending {
		t.Fatalf("an unauthorized call changed the run: %+v", after)
	}
	if data, _, _, _ := c.Log(context.Background(), run.ID, 0); len(data) != 0 {
		t.Fatalf("an unauthorized call wrote the log: %q", data)
	}
}

func TestTrackingAPI_RejectsBadIdsBodiesAndSizes(t *testing.T) {
	d, c := boot(t, home.Dir(t.TempDir()), nil)
	tok := d.Endpoint().Token
	run, err := c.CreateRun(context.Background(), api.CreateRunRequest{Pipeline: "p", Steps: []api.PlanStep{{ID: "a"}}})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name         string
		path, body   string
		want         int
		wantContains string
	}{
		{"unknown id: steps", "/runs/20200101-000000-abcdef/steps", `{"id":"a","status":"running"}`, 404, "not found"},
		{"unknown id: log", "/runs/nope/log?offset=0", "x", 404, "not found"},
		{"unknown id: finish", "/runs/..%2Fx/finish", `{"status":"failed"}`, 404, "not found"},
		{"no log offset", "/runs/" + run.ID + "/log", "x", 400, "offset"},
		{"bad log offset", "/runs/" + run.ID + "/log?offset=-1", "x", 400, "offset"},
		{"log gap", "/runs/" + run.ID + "/log?offset=9", "x", 409, "end"},
		{"log too large", "/runs/" + run.ID + "/log?offset=0", strings.Repeat("x", api.MaxLogChunk+1), 413, ""},
		{"create: not JSON", "/runs", `{`, 400, "read request"},
		{"create: no name", "/runs", `{"pipeline":"","steps":[{"id":"a"}]}`, 400, "pipeline"},
		{"create: no steps", "/runs", `{"pipeline":"p","steps":[]}`, 400, "step"},
		{"create: too many steps", "/runs", `{"pipeline":"p","steps":[` + strings.TrimSuffix(strings.Repeat(`{"id":"s"},`, 2000), ",") + `]}`, 400, "steps"},
		{"create: body over the limit", "/runs", `{"pipeline":"` + strings.Repeat("p", maxJSONBody) + `"}`, 413, ""},
		{"create: name with a control character", "/runs", `{"pipeline":"a\u0007b","steps":[{"id":"a"}]}`, 400, "pipeline"},
		{"create: name too long", "/runs", `{"pipeline":"` + strings.Repeat("n", api.MaxTrackedName+1) + `","steps":[{"id":"a"}]}`, 400, "pipeline"},
		{"create: unknown need", "/runs", `{"pipeline":"p","steps":[{"id":"a","needs":["zzz"]}]}`, 400, "needs"},
		{"finish: reason too long", "/runs/" + run.ID + "/finish", `{"status":"failed","reason":"` + strings.Repeat("r", api.MaxReason+1) + `"}`, 413, ""},
		{"step: unknown step", "/runs/" + run.ID + "/steps", `{"id":"zzz","status":"running"}`, 400, "no step"},
		{"step: bad status", "/runs/" + run.ID + "/steps", `{"id":"a","status":"done"}`, 400, "status"},
		{"finish: bad status", "/runs/" + run.ID + "/finish", `{"status":"running"}`, 400, "status"},
	}
	for _, tc := range cases {
		code, body := call(t, d, tok, "POST", tc.path, []byte(tc.body))
		if code != tc.want || !strings.Contains(body, tc.wantContains) {
			t.Errorf("%s: %d %s, want %d containing %q", tc.name, code, body, tc.want, tc.wantContains)
		}
	}
	// None of that touched the run.
	if after, _ := c.Run(context.Background(), run.ID); after.Status != api.RunRunning || after.Steps[0].Status != api.StepPending {
		t.Fatalf("run = %+v", after)
	}
	if data, _, _, _ := c.Log(context.Background(), run.ID, 0); len(data) != 0 {
		t.Fatalf("log = %q", data)
	}
	// A GET on the write endpoints is not routed to them.
	if code, _ := call(t, d, tok, "GET", "/runs/"+run.ID+"/steps", nil); code == http.StatusOK {
		t.Errorf("GET steps answered %d", code)
	}
}

func TestTrackingAPI_CannotWriteToARunTheDaemonExecutes(t *testing.T) {
	d, c := boot(t, home.Dir(t.TempDir()), nil)
	path := writeDefinition(t, t.TempDir(), "hello.yaml", helloDefinition)
	if _, err := c.Register(context.Background(), api.RegisterRequest{Path: path}); err != nil {
		t.Fatal(err)
	}
	run, err := c.Trigger(context.Background(), "hello")
	if err != nil {
		t.Fatal(err)
	}
	awaitRuns(t, c, "hello", 1)
	tok := d.Endpoint().Token
	for path, body := range map[string]string{
		"/runs/" + run.ID + "/steps":                         `{"id":"greet","status":"failed"}`,
		"/runs/" + run.ID + "/log?offset=0":                  "forged",
		"/runs/" + run.ID + "/finish":                        `{"status":"failed"}`,
		"/runs/" + run.ID + "/log?offset=" + strconv.Itoa(1): "forged",
	} {
		if code, _ := call(t, d, tok, "POST", path, []byte(body)); code != http.StatusConflict {
			t.Errorf("POST %s: %d, want 409", path, code)
		}
	}
	if after, _ := c.Run(context.Background(), run.ID); after.Status != api.RunSucceeded {
		t.Fatalf("run = %+v", after)
	}
}

func TestTrackingAPI_CancelClosesALocalRun(t *testing.T) {
	ctx := context.Background()
	_, c := boot(t, home.Dir(t.TempDir()), nil)
	run, _ := c.CreateRun(ctx, api.CreateRunRequest{Pipeline: "p", Steps: []api.PlanStep{{ID: "a"}}})
	got, err := c.Cancel(ctx, run.ID)
	if err != nil || got.Status != api.RunCancelled {
		t.Fatalf("cancel = %+v, %v", got, err)
	}
	var apiErr *api.StatusError
	if _, _, err := c.AppendLog(ctx, run.ID, 0, nil); err == nil || !errors.As(err, &apiErr) || apiErr.Code != http.StatusConflict {
		t.Fatalf("report after cancel: %v", err)
	}
}
