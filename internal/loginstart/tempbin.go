package loginstart

import "strings"

// temporaryPrefixes are directories whose contents do not outlive a reboot or a cleanup.
var temporaryPrefixes = []string{
	"/tmp/", "/var/tmp/", "/private/tmp/", "/private/var/tmp/",
	"/var/folders/", "/private/var/folders/", "/dev/shm/",
}

// TemporaryReason says why binary looks like a throwaway build, or returns ""
// when it looks like a place to keep a program. A login item that points at a
// deleted file never starts, so install refuses such paths. tempDir is the
// system's temporary directory (os.TempDir).
func TemporaryReason(binary, tempDir string) string {
	p := normalizePath(binary)
	for _, part := range strings.Split(p, "/") {
		if isGoBuildDir(part) {
			return "it was built by go run or go test and is deleted when that command ends"
		}
	}
	if t := normalizePath(tempDir); len(t) > 1 && strings.HasPrefix(p, strings.TrimSuffix(t, "/")+"/") {
		return "it is inside the temporary directory " + tempDir
	}
	for _, prefix := range temporaryPrefixes {
		if strings.HasPrefix(p, prefix) {
			return "it is inside the temporary directory " + strings.TrimSuffix(prefix, "/")
		}
	}
	if strings.Contains(p, "/appdata/local/temp/") || strings.Contains(p, "/windows/temp/") {
		return "it is inside a Windows temporary directory"
	}
	return ""
}

// normalizePath makes paths comparable across platforms: lower case, forward slashes.
func normalizePath(p string) string {
	return strings.ToLower(strings.ReplaceAll(p, `\`, "/"))
}

// isGoBuildDir matches the directories go run and go test build in: "go-build"
// followed by digits, or the build cache directory "go-build" itself.
func isGoBuildDir(name string) bool {
	rest, ok := strings.CutPrefix(name, "go-build")
	if !ok {
		return false
	}
	for _, r := range rest {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
