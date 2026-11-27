package pipeline

import (
	"fmt"
	"path"
	"strings"
)

// Patterns of on.file (spec 029) use forward slashes on every system. A
// pattern without a wildcard names a file or a folder, and a folder covers
// everything under it; a pattern ending in "/" is a folder. "*" and "?" match
// inside one path segment, "**" matches any number of segments.

// CheckPattern reports a pattern that can never match: empty, or with a bad
// [...] class.
func CheckPattern(pattern string) error {
	p := normPattern(pattern)
	if p == "" {
		return fmt.Errorf("a path pattern is empty")
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "**" {
			continue
		}
		if _, err := path.Match(seg, ""); err != nil {
			return fmt.Errorf("path pattern %q: %w", pattern, err)
		}
	}
	return nil
}

// MatchPattern reports whether the slash-separated path rel matches pattern.
func MatchPattern(pattern, rel string) bool {
	p := normPattern(pattern)
	rel = strings.Trim(strings.ReplaceAll(rel, "\\", "/"), "/")
	if !strings.ContainsAny(p, "*?[") {
		return rel == p || strings.HasPrefix(rel, p+"/")
	}
	return matchSegments(strings.Split(p, "/"), strings.Split(rel, "/"))
}

// PatternBase is the part of a pattern before its first wildcard: the folder
// (or file) a watcher has to look at.
func PatternBase(pattern string) string {
	p := normPattern(pattern)
	segs := strings.Split(p, "/")
	for i, seg := range segs {
		if strings.ContainsAny(seg, "*?[") {
			return strings.Join(segs[:i], "/")
		}
	}
	return p
}

func normPattern(pattern string) string {
	p := strings.ReplaceAll(strings.TrimSpace(pattern), "\\", "/")
	p = strings.TrimPrefix(p, "./")
	if strings.HasPrefix(p, "/") {
		return "/" + strings.Trim(p, "/")
	}
	return strings.Trim(p, "/")
}

func matchSegments(pat, segs []string) bool {
	for len(pat) > 0 {
		if pat[0] == "**" {
			if len(pat) == 1 {
				return true
			}
			for i := 0; i <= len(segs); i++ {
				if matchSegments(pat[1:], segs[i:]) {
					return true
				}
			}
			return false
		}
		if len(segs) == 0 {
			return false
		}
		if ok, _ := path.Match(pat[0], segs[0]); !ok {
			return false
		}
		pat, segs = pat[1:], segs[1:]
	}
	return len(segs) == 0
}
