package secrets

import (
	"bytes"
	"strings"
	"testing"
)

func redact(t *testing.T, values []string, chunks ...string) string {
	t.Helper()
	var out bytes.Buffer
	r := NewRedactor(values, func(b []byte) { out.Write(b) })
	for _, c := range chunks {
		if _, err := r.Write([]byte(c)); err != nil {
			t.Fatal(err)
		}
	}
	r.Flush()
	return out.String()
}

func TestRedactor(t *testing.T) {
	const v = "s3cr3t-canary"
	tests := []struct {
		name   string
		values []string
		chunks []string
		want   string
	}{
		{"whole", []string{v}, []string{"token " + v + " end\n"}, "token *** end\n"},
		{"twice", []string{v}, []string{v + v}, "******"},
		{"split in two", []string{v}, []string{"a s3cr3", "t-canary b"}, "a *** b"},
		{"split in many", []string{v}, strings.Split("x "+v+" y", ""), "x *** y"},
		{"partial prefix then other text", []string{v}, []string{"s3cr", "etive"}, "s3cretive"},
		{"prefix held until flush", []string{v}, []string{"ends with s3cr"}, "ends with s3cr"},
		{"multi-line value", []string{"line1\nline2"}, []string{"a line1\nli", "ne2 b"}, "a *** b"},
		{"two values", []string{"aaaa", "bbbb"}, []string{"aaaa-bb", "bb"}, "***-***"},
		{"overlap prefers longer", []string{"abc", "abcd"}, []string{"abc", "d!"}, "***!"},
		{"shorter when longer fails", []string{"abc", "abcd"}, []string{"abc", "x"}, "***x"},
		{"empty values ignored", []string{"", v}, []string{v}, "***"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := redact(t, tt.values, tt.chunks...); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRedactor_ReleasesOutputThatCannotBeASecret(t *testing.T) {
	var out bytes.Buffer
	r := NewRedactor([]string{"s3cr3t-canary"}, func(b []byte) { out.Write(b) })
	_, _ = r.Write([]byte("hello world\n"))
	if out.String() != "hello world\n" {
		t.Errorf("live output delayed: %q", out.String())
	}
}

func TestRedactor_NilWhenNothingToHide(t *testing.T) {
	if NewRedactor(nil, nil) != nil || NewRedactor([]string{""}, nil) != nil {
		t.Error("expected nil Redactor")
	}
}

func TestScrub(t *testing.T) {
	got := Scrub("a abcd b abc c", []string{"abc", "abcd", ""})
	if got != "a *** b *** c" {
		t.Errorf("Scrub = %q", got)
	}
	if Scrub("plain", nil) != "plain" {
		t.Error("Scrub changed text with no values")
	}
}
