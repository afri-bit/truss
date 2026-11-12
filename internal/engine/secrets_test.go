package engine

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/afri-bit/truss/internal/pipeline"
)

// canary is a made-up value, never a real credential.
const canary = "canary-secret-0001"

func store(values map[string]string) Option {
	return WithSecrets(func() (map[string]string, error) { return values, nil })
}

// sink records the output each step produced, as events saw it.
type sink struct {
	mu      sync.Mutex
	out     map[string]string
	started []string
}

func newSink() *sink { return &sink{out: map[string]string{}} }

func (s *sink) StepStarted(id string) {
	s.mu.Lock()
	s.started = append(s.started, id)
	s.mu.Unlock()
}
func (s *sink) StepOutput(id string, c []byte) {
	s.mu.Lock()
	s.out[id] += string(c)
	s.mu.Unlock()
}
func (s *sink) StepStopped(string, string)       {}
func (s *sink) StepFinished(string, *int, error) {}

func secretPipeline(steps ...pipeline.Step) pipeline.Pipeline {
	return pipeline.Pipeline{Version: "1", Name: "secrets", Steps: steps}
}

func TestRun_SecretReachesOnlyDeclaringStep(t *testing.T) {
	p := secretPipeline(
		pipeline.Step{ID: "uses", Run: echoVar("TRUSS_TOKEN") + " && " + echoVar("TRUSS_OTHER"), Secrets: []string{"TRUSS_TOKEN"}},
		pipeline.Step{ID: "plain", Run: echoVar("TRUSS_TOKEN"), Needs: []string{"uses"}},
	)
	ev := newSink()
	// The step must not see the variable through the run environment either.
	base := os.Environ()
	run, err := Run(context.Background(), p, t.TempDir(), base, ev,
		store(map[string]string{"TRUSS_TOKEN": canary, "TRUSS_OTHER": canary + "-other"}))
	if err != nil || run.Status != pipeline.StatusSucceeded {
		t.Fatalf("run = %+v, err = %v", run, err)
	}
	// The value is redacted, which proves the step received TRUSS_TOKEN: an
	// unset variable would print nothing (or, under cmd.exe, "%NAME%").
	if got := strings.Fields(ev.out["uses"]); len(got) == 0 || got[0] != "***" {
		t.Errorf("uses output = %q, want *** first", ev.out["uses"])
	}
	if strings.Contains(ev.out["plain"], "***") || strings.Contains(ev.out["plain"], canary) {
		t.Errorf("an undeclared step received the secret: %q", ev.out["plain"])
	}
}

func TestRun_RedactsEverywhereOutputGoes(t *testing.T) {
	p := secretPipeline(pipeline.Step{ID: "leak", Run: echoVar("TRUSS_TOKEN"), Secrets: []string{"TRUSS_TOKEN"}})
	ev := newSink()
	run, err := Run(context.Background(), p, t.TempDir(), os.Environ(), ev, store(map[string]string{"TRUSS_TOKEN": canary}))
	if err != nil || run.Status != pipeline.StatusSucceeded {
		t.Fatalf("run = %+v, err = %v", run, err)
	}
	for where, text := range map[string]string{"events": ev.out["leak"], "result": run.Steps[0].Output, "stderr": run.Steps[0].Stderr} {
		if strings.Contains(text, canary) {
			t.Errorf("%s contains the secret: %q", where, text)
		}
	}
	if !strings.Contains(ev.out["leak"], "***") || !strings.Contains(run.Steps[0].Output, "***") {
		t.Errorf("no mask: events %q, result %q", ev.out["leak"], run.Steps[0].Output)
	}
}

func TestRun_RedactsAValueSplitAcrossWrites(t *testing.T) {
	half := len(canary) / 2
	p := secretPipeline(pipeline.Step{ID: "split", Run: helperCmd("print-split", canary[:half], canary[half:]), Secrets: []string{"TRUSS_TOKEN"}})
	ev := newSink()
	run, err := Run(context.Background(), p, t.TempDir(), os.Environ(), ev, store(map[string]string{"TRUSS_TOKEN": canary}))
	if err != nil || run.Status != pipeline.StatusSucceeded {
		t.Fatalf("run = %+v, err = %v", run, err)
	}
	if got := ev.out["split"]; got != "***" {
		t.Errorf("events output = %q, want ***", got)
	}
}

func TestRun_UnrelatedStoredValuesAreRedactedToo(t *testing.T) {
	p := secretPipeline(pipeline.Step{ID: "a", Run: echoVar("TRUSS_NOPE"), Secrets: []string{"TRUSS_TOKEN"},
		Env: map[string]string{"TRUSS_NOPE": canary + "-other"}})
	ev := newSink()
	_, err := Run(context.Background(), p, t.TempDir(), os.Environ(), ev,
		store(map[string]string{"TRUSS_TOKEN": canary, "UNUSED": canary + "-other"}))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(ev.out["a"], canary) {
		t.Errorf("output = %q", ev.out["a"])
	}
}

func TestRun_MissingSecretFailsBeforeAnyStep(t *testing.T) {
	p := secretPipeline(
		pipeline.Step{ID: "first", Run: helperCmd("exit", "0")},
		pipeline.Step{ID: "second", Run: helperCmd("exit", "0"), Secrets: []string{"NPM_TOKEN", "OTHER"}, Needs: []string{"first"}},
	)
	for name, opts := range map[string][]Option{
		"empty store": {store(map[string]string{"UNRELATED": canary})},
		"no store":    nil,
	} {
		t.Run(name, func(t *testing.T) {
			ev := newSink()
			_, err := Run(context.Background(), p, t.TempDir(), nil, ev, opts...)
			if err == nil {
				t.Fatal("expected an error")
			}
			msg := err.Error()
			for _, want := range []string{"NPM_TOKEN", "OTHER", "truss secret set"} {
				if !strings.Contains(msg, want) {
					t.Errorf("error %q lacks %q", msg, want)
				}
			}
			if strings.Contains(msg, canary) {
				t.Errorf("error prints a value: %q", msg)
			}
			if len(ev.started) != 0 {
				t.Errorf("steps started: %v", ev.started)
			}
		})
	}
}

func TestRun_UnreadableStoreFailsTheRun(t *testing.T) {
	p := secretPipeline(pipeline.Step{ID: "a", Run: helperCmd("exit", "0"), Secrets: []string{"X"}})
	ev := newSink()
	_, err := Run(context.Background(), p, t.TempDir(), nil, ev,
		WithSecrets(func() (map[string]string, error) { return nil, errors.New("store broken") }))
	if err == nil || !strings.Contains(err.Error(), "store broken") || len(ev.started) != 0 {
		t.Errorf("err = %v, started = %v", err, ev.started)
	}
}

func TestRun_StoreIsNotReadWithoutDeclaredSecrets(t *testing.T) {
	called := false
	p := secretPipeline(pipeline.Step{ID: "a", Run: helperCmd("exit", "0")})
	_, err := Run(context.Background(), p, t.TempDir(), nil, nil,
		WithSecrets(func() (map[string]string, error) { called = true; return nil, errors.New("boom") }))
	if err != nil || called {
		t.Errorf("err = %v, store read = %v", err, called)
	}
}

func TestRun_SecretOverridesEnv(t *testing.T) {
	p := secretPipeline(pipeline.Step{ID: "a", Run: echoVar("TRUSS_TOKEN"),
		Env: map[string]string{"TRUSS_TOKEN": "from-env"}, Secrets: []string{"TRUSS_TOKEN"}})
	ev := newSink()
	if _, err := Run(context.Background(), p, t.TempDir(), os.Environ(), ev, store(map[string]string{"TRUSS_TOKEN": canary})); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(ev.out["a"], "from-env") {
		t.Errorf("env won over the secret: %q", ev.out["a"])
	}
}

func TestRun_RedactsAValueWhenTheOtherStreamInterleaves(t *testing.T) {
	half := len(canary) / 2
	p := secretPipeline(pipeline.Step{ID: "mix", Run: helperCmd("print-interleaved", canary[:half], canary[half:], "warning\n"), Secrets: []string{"TRUSS_TOKEN"}})
	ev := newSink()
	run, err := Run(context.Background(), p, t.TempDir(), os.Environ(), ev, store(map[string]string{"TRUSS_TOKEN": canary}))
	if err != nil || run.Status != pipeline.StatusSucceeded {
		t.Fatalf("run = %+v, err = %v", run, err)
	}
	got := ev.out["mix"]
	if strings.Contains(got, canary[:half]) || strings.Contains(got, canary[half:]) || !strings.Contains(got, "***") || !strings.Contains(got, "warning") {
		t.Errorf("events output = %q", got)
	}
}
