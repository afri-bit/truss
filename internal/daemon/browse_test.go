package daemon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/afri-bit/truss/internal/api"
	"github.com/afri-bit/truss/internal/home"
)

func browseTree(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"zeta", "Alpha", ".hidden"} {
		if err := os.Mkdir(filepath.Join(root, d), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for name, body := range map[string]string{
		"b.yaml": "secret-content-1", "a.YML": "x", "notes.txt": "x", "c.yaml.bak": "x", ".dot.yaml": "x",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func names(l api.DirListing) []string {
	var out []string
	for _, e := range l.Entries {
		out = append(out, e.Kind+":"+e.Name)
	}
	return out
}

func TestBrowse_ListsDirectoriesAndYAMLNamesOnly(t *testing.T) {
	_, c := boot(t, home.Dir(t.TempDir()), nil)
	root := browseTree(t)
	l, err := c.Browse(context.Background(), root, false)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(names(l), ",")
	want := "dir:Alpha,dir:zeta,file:a.YML,file:b.yaml"
	if got != want {
		t.Fatalf("entries = %s, want %s", got, want)
	}
	if l.Path != root || l.Parent != filepath.Dir(root) || len(l.Roots) == 0 {
		t.Fatalf("listing = %+v", l)
	}
	raw, _ := json.Marshal(l)
	if strings.Contains(string(raw), "secret-content") {
		t.Fatal("file content leaked into the listing")
	}
	l, err = c.Browse(context.Background(), root, true)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(names(l), ","); !strings.Contains(got, "dir:.hidden") || !strings.Contains(got, "file:.dot.yaml") {
		t.Fatalf("hidden entries missing: %s", got)
	}
}

func TestBrowse_StartsAtTheUsersHome(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	_, c := boot(t, home.Dir(t.TempDir()), nil)
	l, err := c.Browse(context.Background(), "", false)
	if err != nil {
		t.Fatal(err)
	}
	if l.Path != dir {
		t.Fatalf("started at %q, want %q", l.Path, dir)
	}
}

func TestBrowse_RejectsBadPaths(t *testing.T) {
	_, c := boot(t, home.Dir(t.TempDir()), nil)
	root := browseTree(t)
	file := filepath.Join(root, "b.yaml")
	cases := map[string]struct {
		path string
		code int
	}{
		"relative":          {"relative/dir", 400},
		"dot-dot":           {root + string(filepath.Separator) + ".." + string(filepath.Separator) + "..", 400},
		"dot-dot-slash":     {root + "/../../etc", 400},
		"dot-dot-backslash": {root + `\..\..\etc`, 400},
		"nul":               {root + "\x00/etc", 400},
		"missing":           {filepath.Join(root, "nope"), 404},
		"a file":            {file, 400},
	}
	for name, tc := range cases {
		_, err := c.Browse(context.Background(), tc.path, false)
		var se *api.StatusError
		if err == nil || !asStatus(err, &se) || se.Code != tc.code {
			t.Errorf("%s: err = %v, want status %d", name, err, tc.code)
		}
	}
}

func asStatus(err error, target **api.StatusError) bool {
	se, ok := err.(*api.StatusError) //nolint:errorlint // the client returns it unwrapped
	if ok {
		*target = se
	}
	return ok
}

func TestBrowse_SymlinksAreReportedHonestly(t *testing.T) {
	_, c := boot(t, home.Dir(t.TempDir()), nil)
	root := browseTree(t)
	outside, _ := filepath.EvalSymlinks(t.TempDir())
	if err := os.WriteFile(filepath.Join(outside, "p.yaml"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	links := [][2]string{
		{outside, "link-dir"},
		{filepath.Join(outside, "p.yaml"), "link.yaml"},
		{filepath.Join(root, "gone"), "broken"},
		{filepath.Join(outside, "p.yaml"), "link.txt"},
	}
	for _, l := range links {
		if err := os.Symlink(l[0], filepath.Join(root, l[1])); err != nil {
			if os.Getenv("CI") != "" {
				t.Fatalf("symlinks must work in CI: %v", err)
			}
			t.Skipf("symlinks unavailable: %v", err)
		}
	}

	l, err := c.Browse(context.Background(), root, false)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]api.DirEntry{}
	for _, e := range l.Entries {
		seen[e.Name] = e
	}
	if e := seen["link-dir"]; e.Kind != "dir" || !e.Link {
		t.Errorf("link-dir = %+v", e)
	}
	if e := seen["link.yaml"]; e.Kind != "file" || !e.Link {
		t.Errorf("link.yaml = %+v", e)
	}
	for _, n := range []string{"broken", "link.txt"} {
		if _, ok := seen[n]; ok {
			t.Errorf("%s should not be listed", n)
		}
	}
	// Listing through the link reports where it really is.
	l, err = c.Browse(context.Background(), filepath.Join(root, "link-dir"), false)
	if err != nil {
		t.Fatal(err)
	}
	if l.Path != outside || l.Parent != filepath.Dir(outside) {
		t.Errorf("through link: path %q parent %q, want %q", l.Path, l.Parent, outside)
	}
}

func TestBrowse_RootHasNoParent(t *testing.T) {
	_, c := boot(t, home.Dir(t.TempDir()), nil)
	root := string(filepath.Separator)
	if runtime.GOOS == "windows" {
		root = filepath.VolumeName(os.Getenv("SystemRoot")) + `\`
	}
	l, err := c.Browse(context.Background(), root, false)
	if err != nil {
		t.Fatal(err)
	}
	if l.Parent != "" || l.Path != root {
		t.Fatalf("root listing: path %q parent %q", l.Path, l.Parent)
	}
}

func TestBrowse_RequiresTheToken(t *testing.T) {
	d, _ := boot(t, home.Dir(t.TempDir()), nil)
	u := "http://" + d.Endpoint().Address + api.Prefix + "/fs?path=" + url.QueryEscape(browseTree(t))
	resp, err := http.Get(u)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no token: %d", resp.StatusCode)
	}
}

func TestCheckBrowsePath_Syntax(t *testing.T) {
	bad := []struct{ goos, path string }{
		{"linux", ""}, {"linux", "rel"}, {"linux", "/a/../b"}, {"linux", "/a/\x00"}, {"linux", "/.."},
		{"windows", `C:`}, {"windows", `C:foo`}, {"windows", `C:\a\..\b`}, {"windows", `C:/a/../b`},
		{"windows", `\\server\share`}, {"windows", `//server/share`}, {"windows", `\\?\C:\`}, {"windows", `\\.\PhysicalDrive0`},
		{"windows", `\rooted`}, {"windows", `C:\file:stream`}, {"windows", `C:\x\CON`}, {"windows", `C:\NUL.txt`}, {"windows", `C:\a\com1`}, {"windows", `C:\LPT9. `}, {"windows", `C:\CONIN$`}, {"windows", `1:\`}, {"windows", "C:\\a\x00"},
	}
	for _, tc := range bad {
		if err := checkBrowsePath(tc.path, tc.goos); err == nil {
			t.Errorf("%s %q accepted", tc.goos, tc.path)
		}
	}
	good := []struct{ goos, path string }{
		{"linux", "/"}, {"linux", "/home/u"}, {"linux", "/a..b/c"}, {"windows", `C:\`}, {"windows", `c:\Users\me`}, {"windows", `C:\console`}, {"windows", `C:\com10`}, {"windows", `D:/work`},
	}
	for _, tc := range good {
		if err := checkBrowsePath(tc.path, tc.goos); err != nil {
			t.Errorf("%s %q rejected: %v", tc.goos, tc.path, err)
		}
	}
}

func TestBrowse_RefusesANonLoopbackPeer(t *testing.T) {
	d, _ := boot(t, home.Dir(t.TempDir()), nil)
	req := httptest.NewRequest(http.MethodGet, api.Prefix+"/fs?path="+url.QueryEscape(browseTree(t)), nil)
	req.Host = "127.0.0.1"
	req.RemoteAddr = "192.0.2.1:1234"
	req.Header.Set("Authorization", "Bearer "+d.Endpoint().Token)
	rec := httptest.NewRecorder()
	d.handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("remote peer: %d", rec.Code)
	}
	req.RemoteAddr = "127.0.0.1:1234"
	rec = httptest.NewRecorder()
	d.handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("loopback peer: %d", rec.Code)
	}
}
