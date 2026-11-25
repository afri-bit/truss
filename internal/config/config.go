// Package config reads the daemon's configuration file and resolves each
// setting from a command-line flag, an environment variable, the file or the
// default, remembering where each value came from (spec 027).
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// FileName is the configuration file in the TRUSS home.
const FileName = "config.yaml"

// maxFileBytes bounds the file; it holds a few settings.
const maxFileBytes = 64 << 10

// Sources of a value, as `truss daemon config` shows them.
const (
	SourceDefault = "default"
	SourceFile    = "config.yaml"
	SourceFlag    = "flag"
)

// Setting is one configurable value of the daemon.
type Setting struct {
	// Key is the name in config.yaml, e.g. max_runs.
	Key string
	// Flag is the command-line flag of daemon start and run, without dashes.
	Flag string
	// Env is the environment variable that sets it.
	Env string
	// Default is the value when nothing sets it.
	Default string
	// Help says what it does, for `truss daemon config`.
	Help string
	// check validates a value and returns it normalised.
	check func(string) (string, error)
}

// Settings lists every setting, in the order `truss daemon config` prints them.
var Settings = []Setting{
	{Key: "address", Flag: "address", Env: "TRUSS_DAEMON_ADDRESS", Default: "127.0.0.1:7878",
		Help: "where the daemon listens; loopback only", check: nonEmpty},
	{Key: "max_runs", Flag: "max-runs", Env: "TRUSS_DAEMON_MAX_RUNS", Default: "4",
		Help: "runs that execute at the same time", check: atLeastOne},
	{Key: "history", Flag: "history", Env: "TRUSS_DAEMON_HISTORY", Default: "200",
		Help: "finished runs kept, with their logs; older ones are removed", check: atLeastOne},
}

// Value is a resolved setting.
type Value struct {
	Setting
	Value string
	// Source is "default", "config.yaml", "flag" or the environment variable name.
	Source string
}

// Config is the resolved configuration.
type Config struct {
	// Path is the file that was read; Exists is false when there is none.
	Path   string
	Exists bool
	Values []Value
}

// Get returns the value of a key; it panics on a key that is not a Setting,
// which is a programming error.
func (c Config) Get(key string) string {
	for _, v := range c.Values {
		if v.Key == key {
			return v.Value
		}
	}
	panic("config: unknown key " + key)
}

// Int returns the value of a numeric key.
func (c Config) Int(key string) int {
	n, _ := strconv.Atoi(c.Get(key))
	return n
}

// Resolve reads the file at path (a missing file is fine) and resolves every
// setting: a flag wins over the environment, which wins over the file, which
// wins over the default. flags maps flag names to the values given; env is
// the process environment as KEY=VALUE.
func Resolve(path string, flags map[string]string, env []string) (Config, error) {
	file, exists, err := read(path)
	if err != nil {
		return Config{}, err
	}
	envMap := map[string]string{}
	for _, kv := range env {
		if k, v, ok := strings.Cut(kv, "="); ok {
			envMap[k] = v
		}
	}
	c := Config{Path: path, Exists: exists}
	for _, s := range Settings {
		v := Value{Setting: s, Value: s.Default, Source: SourceDefault}
		if fv, ok := file[s.Key]; ok {
			v.Value, v.Source = fv, SourceFile
		}
		if ev, ok := envMap[s.Env]; ok && ev != "" {
			v.Value, v.Source = ev, s.Env
		}
		if fl, ok := flags[s.Flag]; ok {
			v.Value, v.Source = fl, SourceFlag
		}
		norm, err := s.check(v.Value)
		if err != nil {
			return Config{}, fmt.Errorf("%s (from %s): %w", s.Key, describe(v.Source, s), err)
		}
		v.Value = norm
		c.Values = append(c.Values, v)
	}
	return c, nil
}

func describe(source string, s Setting) string {
	switch source {
	case SourceFlag:
		return "--" + s.Flag
	case SourceFile, SourceDefault:
		return source
	}
	return "the environment variable " + source
}

// read parses the file into key to value. Errors name a key, never a value.
func read(path string) (map[string]string, bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read %s: %w", FileName, err)
	}
	if !info.Mode().IsRegular() {
		return nil, false, fmt.Errorf("%s is not a regular file", FileName)
	}
	if info.Size() > maxFileBytes {
		return nil, false, fmt.Errorf("%s is too large (the limit is %d bytes)", FileName, maxFileBytes)
	}
	data, err := os.ReadFile(path) // #nosec G304 -- the TRUSS home's own file, Lstat-checked above
	if err != nil {
		return nil, false, fmt.Errorf("read %s: %w", FileName, err)
	}
	out := map[string]string{}
	if len(bytes.TrimSpace(data)) == 0 {
		return out, true, nil
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, false, fmt.Errorf("%s: invalid YAML", FileName)
	}
	if len(doc.Content) == 0 {
		return out, true, nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, false, fmt.Errorf("%s must be a mapping of settings", FileName)
	}
	known := map[string]bool{}
	for _, s := range Settings {
		known[s.Key] = true
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		key, v := root.Content[i].Value, root.Content[i+1]
		switch {
		case !known[key]:
			return nil, false, fmt.Errorf("%s: unknown setting %q; known: %s", FileName, key, strings.Join(Keys(), ", "))
		case v.Kind != yaml.ScalarNode:
			return nil, false, fmt.Errorf("%s: %s must be a single value", FileName, key)
		}
		out[key] = v.Value
	}
	return out, true, nil
}

// Keys returns the setting keys, sorted.
func Keys() []string {
	keys := make([]string, 0, len(Settings))
	for _, s := range Settings {
		keys = append(keys, s.Key)
	}
	sort.Strings(keys)
	return keys
}

func nonEmpty(v string) (string, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", errors.New("must not be empty")
	}
	return v, nil
}

func atLeastOne(v string) (string, error) {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n < 1 {
		return "", errors.New("must be a whole number of at least 1")
	}
	return strconv.Itoa(n), nil
}
