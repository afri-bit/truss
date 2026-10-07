package main

import (
	"bytes"
	"errors"
	"testing"
)

func TestWriteVersion(t *testing.T) {
	tests := []struct {
		name string
		in   buildInfo
		want string
	}{
		{
			name: "release build",
			in:   buildInfo{Version: "v0.1.0", Commit: "abc1234", Date: "2026-01-01T00:00:00Z", GoVersion: "go1.27.0", Platform: "linux/amd64"},
			want: "truss v0.1.0\ncommit: abc1234\nbuilt: 2026-01-01T00:00:00Z\ngo: go1.27.0\nplatform: linux/amd64\n",
		},
		{
			name: "development build",
			in:   buildInfo{Version: "dev", Commit: "none", Date: "unknown", GoVersion: "go1.27.0", Platform: "windows/arm64"},
			want: "truss dev\ncommit: none\nbuilt: unknown\ngo: go1.27.0\nplatform: windows/arm64\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := writeVersion(&buf, tt.in); err != nil {
				t.Fatalf("writeVersion() error = %v", err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("writeVersion() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("boom") }

func TestWriteVersionPropagatesError(t *testing.T) {
	if err := writeVersion(failingWriter{}, buildInfo{}); err == nil {
		t.Fatal("writeVersion() error = nil, want non-nil")
	}
}

func TestCurrentBuildInfoUsesLinkTimeValues(t *testing.T) {
	saved := [3]string{version, commit, date}
	t.Cleanup(func() { version, commit, date = saved[0], saved[1], saved[2] })
	version, commit, date = "v9.9.9", "deadbeef", "2026-02-03T04:05:06Z"

	bi := currentBuildInfo()
	if bi.Version != "v9.9.9" || bi.Commit != "deadbeef" || bi.Date != "2026-02-03T04:05:06Z" {
		t.Errorf("currentBuildInfo() = %+v, want link-time values", bi)
	}
	if bi.GoVersion == "" || bi.Platform == "" {
		t.Errorf("currentBuildInfo() = %+v, want runtime fields populated", bi)
	}
}
