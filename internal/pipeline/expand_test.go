package pipeline

import (
	"strings"
	"testing"
)

func host(vars map[string]string) func(string) string {
	return func(name string) string { return vars[name] }
}

func TestExpand(t *testing.T) {
	look := host(map[string]string{"HOME": "/home/me", "EMPTY": ""})
	for in, want := range map[string]string{
		"plain":          "plain",
		"${HOME}/bin":    "/home/me/bin",
		"a$$b":           "a$b",
		"$HOME stays":    "$HOME stays",
		"${UNSET}x":      "x",
		"${EMPTY}":       "",
		"cost: 5$":       "cost: 5$",
		"$${HOME}":       "${HOME}",
		"${HOME}${HOME}": "/home/me/home/me",
	} {
		got, err := Expand(in, look)
		if err != nil || got != want {
			t.Errorf("Expand(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"${HOME", "${}", "${1A}", "${A-B}"} {
		if _, err := Expand(bad, look); err == nil {
			t.Errorf("Expand(%q) succeeded", bad)
		}
		if CheckTemplate(bad) == nil {
			t.Errorf("CheckTemplate(%q) = nil", bad)
		}
	}
}

func TestExpandLayers(t *testing.T) {
	base := host(map[string]string{"PATH": "/usr/bin", "HOME": "/home/me"})
	project := map[string]string{"ROOT": "${HOME}/work"}
	flow := map[string]string{"BIN": "${APP}/bin", "APP": "${ROOT}/app", "PATH": "${BIN}:${PATH}"}
	step := map[string]string{"PATH": "/opt:${PATH}"}
	got, err := ExpandLayers(base, project, flow, step)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"ROOT": "/home/me/work",
		"APP":  "/home/me/work/app",
		"BIN":  "/home/me/work/app/bin",
		"PATH": "/opt:/home/me/work/app/bin:/usr/bin",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
	if len(got) != len(want) {
		t.Errorf("got %v", got)
	}
}

func TestExpandLayers_LoopIsAnError(t *testing.T) {
	_, err := ExpandLayers(host(nil), map[string]string{"A": "${B}", "B": "${A}"})
	if err == nil || !strings.Contains(err.Error(), "loop") {
		t.Fatalf("err = %v", err)
	}
}

func TestReserved(t *testing.T) {
	for name, want := range map[string]bool{
		"TRUSS_RUN_ID": true, "TRUSS_FLOW": true, "truss_step": true, "TRUSS_PIPELINE": true,
		"TRUSS_DATA_DIR": false, "TRUSS_CACHE_DIR": false, "TRUSS_TOKEN": false, "MY_TRUSS_X": false,
	} {
		if Reserved(name) != want {
			t.Errorf("Reserved(%q) = %v", name, !want)
		}
	}
}

// On Windows names that differ only in case are one variable: the higher
// layer wins and a self-reference sees the lower layer whatever its case.
func TestExpandLayers_FoldsCaseWhereTheHostDoes(t *testing.T) {
	base := func(name string) string { // a Windows host: case does not matter
		if strings.EqualFold(name, "PATH") {
			return "/sys"
		}
		return ""
	}
	got, err := expandLayers(true, base,
		map[string]string{"PATH": "/tools:${PATH}"},
		map[string]string{"Path": "/x:${Path}", "dir": "${PATH}"},
		map[string]string{"PATH": "/override"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got["PATH"] != "/override" || got["dir"] != "/x:/tools:/sys" {
		t.Fatalf("got %v", got)
	}
	// Without folding they are two variables.
	got, err = expandLayers(false, host(map[string]string{"Path": "/sys"}), map[string]string{"PATH": "a"}, map[string]string{"Path": "b:${Path}"})
	if err != nil || got["PATH"] != "a" || got["Path"] != "b:/sys" {
		t.Fatalf("no fold: %v %v", got, err)
	}
}

func TestExpand_ErrorNeverQuotesTheValue(t *testing.T) {
	err := CheckTemplate("key=${sk-live-abc123}")
	if err == nil || strings.Contains(err.Error(), "sk-live") {
		t.Fatalf("err = %v", err)
	}
}
