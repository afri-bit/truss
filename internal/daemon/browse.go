package daemon

import (
	"errors"
	"fmt"
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

// maxBrowseEntries bounds one directory listing.
const maxBrowseEntries = 5000

// statusError carries an HTTP status for an error that is not one of the
// domain errors statusFor knows.
type statusError struct {
	code int
	msg  string
}

func (e *statusError) Error() string { return e.msg }

// browse lists one directory for the register dialog: names of directories
// and of .yaml/.yml files, never content, size or time. See ADR 0006.
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
	info, err := os.Stat(resolved)
	if err != nil {
		return browseError(path, err)
	}
	if !info.IsDir() {
		return badRequest("%q is not a directory", path)
	}
	dirents, err := os.ReadDir(resolved)
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
		// Stat follows a link to its target; a broken link fails and is left out.
		st, err := os.Stat(filepath.Join(resolved, name))
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
	if len(list.Entries) > maxBrowseEntries {
		list.Entries, list.Truncated = list.Entries[:maxBrowseEntries], true
	}
	return writeJSON(w, http.StatusOK, list)
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
