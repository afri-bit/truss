package daemon

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/afri-bit/truss/internal/api"
)

// maxBrowseEntries bounds one directory listing; maxBrowseScan bounds how many
// names are read from a directory before the listing stops.
const (
	maxBrowseEntries = 5000
	maxBrowseScan    = 50000
)

// statusError carries an HTTP status for an error that is not one of the
// domain errors statusFor knows.
type statusError struct {
	code int
	msg  string
}

func (e *statusError) Error() string { return e.msg }

// browse lists one directory for the register dialog: names of directories
// and of .yaml/.yml files, never content, size or time. See ADR 0010.
func (d *Daemon) browse(w http.ResponseWriter, r *http.Request) error {
	if !loopbackPeer(r.RemoteAddr) {
		return &statusError{http.StatusForbidden, "the directory listing is only available to this machine"}
	}
	path := r.URL.Query().Get("path")
	if path == "" {
		path = startDir()
	}
	if err := checkBrowsePath(path, runtime.GOOS); err != nil {
		return badRequest("%s", err.Error())
	}
	if !filepath.IsAbs(path) {
		return badRequest("path must be absolute, got %q", path)
	}
	// Resolve links up front so the answer says where the listing really is,
	// and so "up" from a linked directory goes to its resolved parent.
	resolved, err := filepath.EvalSymlinks(filepath.Clean(path))
	if err != nil {
		return browseError(path, err)
	}
	info, err := os.Stat(resolved) // #nosec G703 -- loopback-only, checkBrowsePath-validated absolute path (ADR 0010)
	if err != nil {
		return browseError(path, err)
	}
	if !info.IsDir() {
		return badRequest("%q is not a directory", path)
	}
	dirents, scanCut, err := readNames(r.Context(), resolved)
	if err != nil {
		return browseError(path, err)
	}
	showHidden := r.URL.Query().Get("hidden") == "true"
	list := api.DirListing{Path: resolved, Roots: browseRoots(), Entries: []api.DirEntry{}}
	if parent := filepath.Dir(resolved); parent != resolved {
		list.Parent = parent
	}
	for _, de := range dirents {
		name := de.Name()
		if !showHidden && strings.HasPrefix(name, ".") {
			continue
		}
		entry := api.DirEntry{Name: name, Link: de.Type()&(fs.ModeSymlink|fs.ModeIrregular) != 0} // Windows junctions are irregular
		// Never touch a link that points at a network or device path: Windows
		// would open a connection and send credentials to it.
		if entry.Link && networkLink(filepath.Join(resolved, name)) {
			continue
		}
		// Stat follows a link to its target; a broken link fails and is left out.
		st, err := os.Stat(filepath.Join(resolved, name)) // #nosec G703 -- an entry of the validated directory above
		if err != nil {
			continue
		}
		switch {
		case st.IsDir():
			entry.Kind = "dir"
		case st.Mode().IsRegular() && isYAMLName(name):
			entry.Kind = "file"
		default:
			continue
		}
		list.Entries = append(list.Entries, entry)
	}
	sort.Slice(list.Entries, func(i, j int) bool {
		a, b := list.Entries[i], list.Entries[j]
		if a.Kind != b.Kind {
			return a.Kind == "dir"
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
	list.Truncated = scanCut
	if len(list.Entries) > maxBrowseEntries {
		list.Entries, list.Truncated = list.Entries[:maxBrowseEntries], true
	}
	return writeJSON(w, http.StatusOK, list)
}

// readNames reads directory entries in batches, so a huge folder costs a
// bounded amount of memory and time. cut is true when it stopped early.
func readNames(ctx context.Context, dir string) (names []os.DirEntry, cut bool, err error) {
	f, err := os.Open(dir) // #nosec G304 G703 -- a listing of a directory the user chose; only names are read
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = f.Close() }()
	for len(names) < maxBrowseScan {
		batch, err := f.ReadDir(1000)
		names = append(names, batch...)
		if errors.Is(err, io.EOF) {
			return names, false, nil
		}
		if err != nil {
			return nil, false, err
		}
		if ctx.Err() != nil {
			return nil, false, ctx.Err()
		}
	}
	return names, true, nil
}

// networkLink reports whether a link's target is a UNC, device or extended
// path. Such targets only exist on Windows.
func networkLink(link string) bool {
	if runtime.GOOS != "windows" {
		return false
	}
	target, err := os.Readlink(link)
	if err != nil {
		return false
	}
	return strings.HasPrefix(target, `\\`) || strings.HasPrefix(target, "//")
}

func isYAMLName(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".yaml" || ext == ".yml"
}

// startDir is where the dialog opens: the user's home, or the file system root.
func startDir() string {
	if h, err := os.UserHomeDir(); err == nil && filepath.IsAbs(h) {
		return h
	}
	return filepath.VolumeName(os.TempDir()) + string(filepath.Separator)
}

// browseRoots lists the places a listing can start from.
func browseRoots() []string {
	if runtime.GOOS != "windows" {
		return []string{"/"}
	}
	var roots []string
	for letter := 'A'; letter <= 'Z'; letter++ {
		root := string(letter) + `:\`
		if _, err := os.Stat(root); err == nil {
			roots = append(roots, root)
		}
	}
	return roots
}

func browseError(path string, err error) error {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return &statusError{http.StatusNotFound, fmt.Sprintf("%q does not exist", path)}
	case errors.Is(err, fs.ErrPermission):
		return &statusError{http.StatusForbidden, fmt.Sprintf("no permission to read %q", path)}
	}
	return badRequest("cannot list %q", path)
}

func loopbackPeer(remote string) bool {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// checkBrowsePath rejects a path by its spelling, before the file system is
// touched. goos is a parameter so the Windows rules are tested everywhere.
//
// It refuses any ".." element (written with either separator), NUL bytes,
// and on Windows everything but "X:\..." or "X:/...": drive-relative paths,
// UNC paths, and the \\?\ and \\.\ device prefixes.
func checkBrowsePath(p, goos string) error {
	if p == "" {
		return errors.New("path is empty")
	}
	if strings.ContainsRune(p, 0) {
		return errors.New("path contains a NUL byte")
	}
	for _, el := range strings.FieldsFunc(p, func(r rune) bool { return r == '/' || r == '\\' }) {
		if el == ".." {
			return errors.New("path must not contain '..'")
		}
	}
	if goos == "windows" {
		if len(p) < 3 {
			return fmt.Errorf("path must be absolute and start with a drive such as C:\\, got %q", p)
		}
		for _, el := range strings.FieldsFunc(p[2:], func(r rune) bool { return r == '/' || r == '\\' }) {
			if reservedWindowsName(el) {
				return fmt.Errorf("path contains the reserved name %q", el)
			}
		}
		isLetter := len(p) >= 3 && (p[0]|0x20 >= 'a' && p[0]|0x20 <= 'z')
		if !isLetter || p[1] != ':' || (p[2] != '\\' && p[2] != '/') {
			return fmt.Errorf("path must be absolute and start with a drive such as C:\\, got %q", p)
		}
		if strings.Contains(p[2:], ":") {
			return errors.New("path must not contain ':' after the drive")
		}
		return nil
	}
	if !strings.HasPrefix(p, "/") {
		return fmt.Errorf("path must be absolute, got %q", p)
	}
	return nil
}

// reservedWindowsName reports whether a path element names a Windows device
// (CON, NUL, COM1, ...), with or without an extension or trailing dots and spaces.
func reservedWindowsName(el string) bool {
	base := strings.TrimRight(el, ". ")
	if i := strings.IndexByte(base, '.'); i >= 0 {
		base = base[:i]
	}
	base = strings.ToUpper(strings.TrimRight(base, " "))
	switch base {
	case "CON", "PRN", "AUX", "NUL", "CONIN$", "CONOUT$":
		return true
	}
	if len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) {
		return base[3] >= '0' && base[3] <= '9'
	}
	return false
}
