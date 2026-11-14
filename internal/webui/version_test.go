package webui

import (
	"regexp"
	"testing"
)

// The release workflow compares this value with the tag, so it must be a plain semantic version.
func TestVersion_IsSemanticVersion(t *testing.T) {
	semver := regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$`)
	if v := Version(); !semver.MatchString(v) {
		t.Fatalf("Version() = %q, want MAJOR.MINOR.PATCH with an optional -prerelease", v)
	}
}
