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

// maxDoubleStars bounds the "**" of one pattern: each one multiplies the
// work of matching a path.
const maxDoubleStars = 3

// CheckPattern reports a pattern that can never match or that would watch
// too much: empty, a bad [...] class, a filesystem root, "~" (not expanded),
// or more than three "**".
func CheckPattern(pattern string) error {
	p := normPattern(pattern)
	switch {
	case p == "":
		return fmt.Errorf("a path pattern is empty")
	case strings.HasPrefix(strings.TrimSpace(pattern), "~"):
		return fmt.Errorf("path pattern %q: ~ is not expanded; write the folder in full", pattern)
	case isRoot(p, PatternBase(pattern)):
		return fmt.Errorf("path pattern %q would watch a whole disk; name a folder", pattern)
	case p == ".." || strings.HasPrefix(p, "../") || strings.Contains(p, "/../") || strings.HasSuffix(p, "/.."):
		return fmt.Errorf("path pattern %q goes up with ..; write the folder as an absolute path", pattern)
	case strings.Count(p, "**") > maxDoubleStars:
		return fmt.Errorf("path pattern %q has more than %d **", pattern, maxDoubleStars)
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

// isRoot reports a pattern whose base is "/" or a drive such as "C:".
func isRoot(p, base string) bool {
	if strings.HasPrefix(p, "/") && (base == "" || base == "/") {
		return true
	}
	b := strings.TrimSuffix(base, "/")
	return len(b) == 2 && b[1] == ':'
}

// MatchPattern reports whether the slash-separated path rel matches pattern.
// A relative pattern matches a path relative to the project root, an
// absolute one an absolute path.
func MatchPattern(pattern, rel string) bool {
	p := normPattern(pattern)
	rel = strings.TrimPrefix(strings.ReplaceAll(rel, "\\", "/"), "./")
	if strings.HasPrefix(p, "/") != strings.HasPrefix(rel, "/") {
		return false
	}
	p, rel = strings.Trim(p, "/"), strings.Trim(rel, "/")
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
	for strings.Contains(p, "**/**") {
		p = strings.ReplaceAll(p, "**/**", "**")
	}
	if strings.HasPrefix(p, "/") {
		return "/" + strings.Trim(p, "/")
	}
	return strings.Trim(p, "/")
}

// matchSegments matches with memoisation, so "**" costs at most
// len(pat) x len(segs) steps.
func matchSegments(pat, segs []string) bool {
	memo := map[[2]int]bool{}
	var match func(i, j int) bool
	match = func(i, j int) bool {
		key := [2]int{i, j}
		if v, ok := memo[key]; ok {
			return v
		}
		var r bool
		switch {
		case i == len(pat):
			r = j == len(segs)
		case pat[i] == "**":
			r = match(i+1, j) || j < len(segs) && match(i, j+1)
		case j == len(segs):
			r = false
		default:
			ok, _ := path.Match(pat[i], segs[j])
			r = ok && match(i+1, j+1)
		}
		memo[key] = r
		return r
	}
	return match(0, 0)
}
