package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, text string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func source(c Config, key string) string {
	for _, v := range c.Values {
		if v.Key == key {
			return v.Source
		}
	}
	return ""
}

func TestResolve_DefaultsWithoutAFile(t *testing.T) {
	c, err := Resolve(filepath.Join(t.TempDir(), FileName), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.Exists || c.Get("address") != "127.0.0.1:7878" || c.Int("max_runs") != 4 || c.Int("history") != 200 {
		t.Fatalf("config = %+v", c)
	}
	if source(c, "history") != SourceDefault {
		t.Fatalf("source = %q", source(c, "history"))
	}
}

func TestResolve_FlagOverEnvironmentOverFileOverDefault(t *testing.T) {
	path := write(t, "address: 127.0.0.1:9000\nmax_runs: 2\nhistory: 30\n")
	c, err := Resolve(path, map[string]string{"max-runs": "8"}, []string{"TRUSS_DAEMON_HISTORY=60", "TRUSS_DAEMON_MAX_RUNS=5"})
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string][2]string{
		"address":  {"127.0.0.1:9000", SourceFile},
		"max_runs": {"8", SourceFlag},
		"history":  {"60", "TRUSS_DAEMON_HISTORY"},
	} {
		if c.Get(key) != want[0] || source(c, key) != want[1] {
			t.Errorf("%s = %q from %q, want %q from %q", key, c.Get(key), source(c, key), want[0], want[1])
		}
	}
	if !c.Exists {
		t.Error("Exists = false")
	}
}

func TestResolve_RejectsBadInputAndNamesIt(t *testing.T) {
	for text, want := range map[string]string{
		"max_run: 2\n":   `unknown setting "max_run"`,
		"history: 0\n":   "history (from config.yaml): must be a whole number of at least 1",
		"address: [a]\n": "address must be a single value",
		"- a\n":          "must be a mapping",
		"history: [oops": "invalid YAML",
	} {
		if _, err := Resolve(write(t, text), nil, nil); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: err = %v, want %q", text, err, want)
		}
	}
	if _, err := Resolve(write(t, ""), nil, []string{"TRUSS_DAEMON_MAX_RUNS=x"}); err == nil ||
		!strings.Contains(err.Error(), "the environment variable TRUSS_DAEMON_MAX_RUNS") {
		t.Errorf("env: err = %v", err)
	}
	if _, err := Resolve(write(t, ""), map[string]string{"history": "-1"}, nil); err == nil || !strings.Contains(err.Error(), "--history") {
		t.Errorf("flag: err = %v", err)
	}
}

func TestResolve_AnEmptyFileIsFine(t *testing.T) {
	c, err := Resolve(write(t, "\n"), nil, nil)
	if err != nil || !c.Exists || source(c, "address") != SourceDefault {
		t.Fatalf("config = %+v, err = %v", c, err)
	}
}
