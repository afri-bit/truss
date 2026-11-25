// Package project knows the TRUSS project layout: a truss.yaml that marks the
// project, pipelines in .truss/flows/, and generated files in .truss/state/.
//
// It reads the project file, lists and resolves pipeline files and computes
// state paths. It starts nothing and prints nothing, and the pipeline package
// stays unaware of it (specs/016-project-layout).
package project

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/afri-bit/truss/internal/pipeline"
	"github.com/afri-bit/truss/internal/registry"
	"gopkg.in/yaml.v3"
)

const (
	// FileName is the project file in the project root.
	FileName = "truss.yaml"
	// MaxFileBytes is the most TRUSS reads from the project file.
	MaxFileBytes = 1 << 20
	// MaxFlowEntries is the most entries .truss/flows may hold.
	MaxFlowEntries = 256

	trussDir = ".truss"
	flowsDir = "flows"
	stateDir = "state"
)

// ErrNoProject means the folder has no truss.yaml.
var ErrNoProject = errors.New("no project file")

// Project is the metadata of a TRUSS project and where it lives.
type Project struct {
	// Root is the folder that holds truss.yaml: the working directory.
	Root        string
	Name        string
	Description string
	// Env is the project-wide env of truss.yaml (spec 026): variables every
	// flow of the project gets, below the flow's own env.
	Env map[string]string
}

// Open reads and validates the truss.yaml in root. It never looks in parent
// folders. A missing file is ErrNoProject and the error names the folder.
func Open(root string) (Project, error) {
	path := filepath.Join(root, FileName)
	info, err := os.Lstat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return Project{}, fmt.Errorf("%w: there is no %s in %s (run TRUSS from the project folder)", ErrNoProject, FileName, root)
	case err != nil:
		return Project{}, fmt.Errorf("read %s: %w", FileName, err)
	case !info.Mode().IsRegular():
		return Project{}, fmt.Errorf("%s in %s is a link or not a regular file; TRUSS does not follow links", FileName, root)
	}
	data, err := readRegular(path)
	if err != nil {
		return Project{}, fmt.Errorf("read %s: %w", FileName, err)
	}
	p, err := parseMetadata(data)
	if err != nil {
		return Project{}, fmt.Errorf("%s: %w", FileName, err)
	}
	p.Root = root
	return p, nil
}

// readRegular opens path once, requires the open file to be a regular file and
// reads it through that handle, up to MaxFileBytes.
func readRegular(path string) ([]byte, error) {
	// A named pipe blocks the open, so look before opening.
	if info, err := os.Lstat(path); err != nil {
		return nil, err
	} else if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", filepath.Base(path))
	}
	f, err := os.Open(path) // #nosec G304 -- Open Lstat-checked this path
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", filepath.Base(path))
	}
	data, err := io.ReadAll(io.LimitReader(f, MaxFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxFileBytes {
		return nil, fmt.Errorf("%s is too large (the limit is %d bytes)", filepath.Base(path), MaxFileBytes)
	}
	return data, nil
}

// parseMetadata decodes the project file. Errors name a field, never a value.
func parseMetadata(data []byte) (Project, error) {
	var doc yaml.Node
	if len(bytes.TrimSpace(data)) > 0 {
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return Project{}, errors.New("invalid YAML")
		}
	}
	var root *yaml.Node
	if len(doc.Content) > 0 {
		root = doc.Content[0]
		if root.Kind != yaml.MappingNode {
			return Project{}, errors.New("the file must be a mapping with version and project")
		}
	}
	fields, err := mappingFields(root, "at top level", map[string]bool{"version": true, "project": true, "env": true})
	if err != nil {
		return Project{}, err
	}
	version, ok := fields["version"]
	if !ok || version.Kind != yaml.ScalarNode || version.Tag != "!!str" || version.Value != "1" {
		return Project{}, errors.New(`version must be the text "1"`)
	}
	block, ok := fields["project"]
	if !ok || block.Kind != yaml.MappingNode {
		return Project{}, errors.New("project must be a mapping with a name (project.name)")
	}
	meta, err := mappingFields(block, "in project", map[string]bool{"name": true, "description": true})
	if err != nil {
		return Project{}, err
	}
	name, ok := meta["name"]
	if !ok || name.Kind != yaml.ScalarNode || name.Value == "" {
		return Project{}, errors.New("project.name is required")
	}
	if registry.CheckName(name.Value) != nil {
		return Project{}, errors.New("project.name must start with a letter or digit and use only letters, digits, '.', '_' and '-'")
	}
	p := Project{Name: name.Value}
	if d, ok := meta["description"]; ok {
		if d.Kind != yaml.ScalarNode {
			return Project{}, errors.New("project.description must be text")
		}
		p.Description = d.Value
	}
	if e, ok := fields["env"]; ok {
		env, err := parseEnv(e)
		if err != nil {
			return Project{}, err
		}
		p.Env = env
	}
	return p, nil
}

// mappingFields returns the values of a mapping by key. Keys outside allowed
// are an error that names the key and where it is; so is a repeated key.
// steps and jobs at the top level get the message that pipelines live in
// .truss/flows/.
func mappingFields(n *yaml.Node, where string, allowed map[string]bool) (map[string]*yaml.Node, error) {
	out := map[string]*yaml.Node{}
	if n == nil {
		return out, nil
	}
	if where == "at top level" {
		// A pipeline pasted into truss.yaml is the likely mistake; say so first.
		for i := 0; i+1 < len(n.Content); i += 2 {
			if key := n.Content[i].Value; key == "steps" || key == "jobs" {
				return nil, errors.New("this file is project metadata and holds no " + key + "; flows live in .truss/flows/")
			}
		}
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		key := n.Content[i].Value
		if !allowed[key] {
			return nil, fmt.Errorf("unknown field %q %s", key, where)
		}
		if _, dup := out[key]; dup {
			return nil, fmt.Errorf("duplicate field %q %s", key, where)
		}
		out[key] = n.Content[i+1]
	}
	return out, nil
}

// Flow is one pipeline file in .truss/flows/.
type Flow struct {
	// Name is the file stem, the flow name.
	Name string
	// File is the file name with its extension.
	File string
	// Path is the file's path under the project root.
	Path string
}

// NotFoundError means no flow has the requested name.
type NotFoundError struct {
	Name      string
	Available []string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("no flow named %q in .truss/flows; %s", e.Name, AvailableText(e.Available))
}

// AvailableText says which pipeline names exist, for error messages.
func AvailableText(names []string) string {
	if len(names) == 0 {
		return "there are no flows (put <name>.yaml files in .truss/flows/)"
	}
	return "available flows: " + strings.Join(names, ", ")
}

// Names returns the names of flows, in order.
func Names(flows []Flow) []string {
	names := make([]string, len(flows))
	for i, f := range flows {
		names[i] = f.Name
	}
	return names
}

// realDirState Lstats path: whether it exists, and an error when it is a link
// or anything but a directory.
func realDirState(path string) (bool, error) {
	info, err := os.Lstat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return false, nil
	case err != nil:
		return false, err
	case info.Mode()&(fs.ModeSymlink|fs.ModeIrregular) != 0:
		return false, fmt.Errorf("%s is a link or other reparse point; TRUSS does not follow links", path)
	case !info.IsDir():
		return false, fmt.Errorf("%s is not a directory", path)
	}
	return true, nil
}

// Flows lists the pipeline files of the project, sorted by name. A project
// with no .truss or no .truss/flows has no flows, which is not an error. A
// file that clashes, has an unusable name, or too many entries is.
func (p Project) Flows() ([]Flow, error) {
	truss := filepath.Join(p.Root, trussDir)
	dir := filepath.Join(truss, flowsDir)
	for _, d := range []string{truss, dir} {
		ok, err := realDirState(d)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, nil
		}
	}
	f, err := os.Open(dir) // #nosec G304 -- Lstat showed a real folder
	if err != nil {
		return nil, fmt.Errorf("list flows: %w", err)
	}
	defer func() { _ = f.Close() }()
	entries, err := f.ReadDir(MaxFlowEntries + 1)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("list flows: %w", err)
	}
	if len(entries) > MaxFlowEntries {
		return nil, fmt.Errorf(".truss/flows holds more than %d entries; keep flows in a smaller folder", MaxFlowEntries)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	var flows []Flow
	var files []string
	for _, e := range entries {
		file := e.Name()
		stem, ok := flowStem(file)
		if !ok || strings.HasPrefix(file, ".") || !e.Type().IsRegular() {
			continue
		}
		if err := checkFlowStem(stem); err != nil {
			return nil, fmt.Errorf("%s in .truss/flows: %w", file, err)
		}
		files = append(files, file)
		flows = append(flows, Flow{Name: stem, File: file, Path: filepath.Join(dir, file)})
	}
	if err := checkClashes(files); err != nil {
		return nil, err
	}
	return flows, nil
}

// checkClashes is an error naming two files (in sorted order) that would be one
// pipeline: the same stem up to case, as build.yaml and build.yml or Build.yaml.
func checkClashes(files []string) error {
	seen := map[string]string{} // lower-cased stem -> file
	for _, file := range files {
		stem, _ := flowStem(file)
		key := strings.ToLower(stem)
		if other, dup := seen[key]; dup {
			return fmt.Errorf("%s and %s in .truss/flows have the same flow name; keep one", other, file)
		}
		seen[key] = file
	}
	return nil
}

// flowStem returns the stem of a .yaml or .yml file name (case-sensitive).
func flowStem(file string) (string, bool) {
	for _, ext := range []string{".yaml", ".yml"} {
		if stem, ok := strings.CutSuffix(file, ext); ok && stem != "" {
			return stem, true
		}
	}
	return "", false
}

func checkFlowStem(stem string) error {
	if registry.CheckName(stem) != nil {
		return errors.New("the name must start with a letter or digit and use only letters, digits, '.', '_' and '-'")
	}
	return checkStemSafe(stem)
}

// checkStemSafe refuses what some host cannot use as a file or folder name: a
// trailing dot or space and the Windows device names.
func checkStemSafe(stem string) error {
	if strings.HasSuffix(stem, ".") || strings.HasSuffix(stem, " ") {
		return errors.New("the name must not end in a dot or a space")
	}
	// Windows treats CON.txt like CON, so the part before the first dot counts.
	base, _, _ := strings.Cut(stem, ".")
	up := strings.ToUpper(strings.TrimRight(base, " "))
	switch up {
	case "CON", "PRN", "AUX", "NUL", "CONIN$", "CONOUT$":
		return errors.New("the name is a Windows device name")
	}
	if len(up) == 4 && (strings.HasPrefix(up, "COM") || strings.HasPrefix(up, "LPT")) && up[3] >= '1' && up[3] <= '9' {
		return errors.New("the name is a Windows device name")
	}
	return nil
}

// Resolve finds the flow with the given name (case-sensitive). The error is a
// *NotFoundError that lists what exists.
func (p Project) Resolve(name string) (Flow, error) {
	flows, err := p.Flows()
	if err != nil {
		return Flow{}, err
	}
	for _, f := range flows {
		if f.Name == name {
			return f, nil
		}
	}
	return Flow{}, &NotFoundError{Name: name, Available: Names(flows)}
}

// IsPath reports whether a command argument is a path rather than a flow
// name: it holds a slash or backslash or ends in .yaml or .yml.
func IsPath(arg string) bool {
	return strings.ContainsAny(arg, `/\`) || strings.HasSuffix(arg, ".yaml") || strings.HasSuffix(arg, ".yml")
}

// FileStem returns the file name of path without its extension.
func FileStem(path string) string {
	base := filepath.Base(path)
	for _, ext := range []string{".yaml", ".yml"} {
		if stem, ok := strings.CutSuffix(base, ext); ok {
			return stem
		}
	}
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// State is where a local run of one pipeline keeps its generated files.
type State struct {
	// Truss is <root>/.truss and StateDir is <root>/.truss/state.
	Truss, StateDir string
	// Dir is <root>/.truss/state/<flow>.
	Dir string
	// Log, Data and Cache are the run log, TRUSS_DATA_DIR and TRUSS_CACHE_DIR.
	Log, Data, Cache string
}

// StateFor computes the state paths of flow under root. The flow must be
// usable as a folder name on every host.
func StateFor(root, flow string) (State, error) {
	switch {
	case flow == "" || flow == "." || flow == "..":
		return State{}, fmt.Errorf("the flow file name %q cannot name a state folder", flow)
	case strings.ContainsAny(flow, "/\\<>:\"|?*\x00") || strings.ContainsFunc(flow, func(r rune) bool { return r < ' ' }):
		return State{}, errors.New("the flow file name has characters that cannot be used in a state folder name")
	}
	if err := checkStemSafe(flow); err != nil {
		return State{}, fmt.Errorf("the flow file name cannot name a state folder: %w", err)
	}
	truss := filepath.Join(root, trussDir)
	state := filepath.Join(truss, stateDir)
	dir := filepath.Join(state, flow)
	data := filepath.Join(dir, "data")
	return State{Truss: truss, StateDir: state, Dir: dir, Log: filepath.Join(dir, "run.log"), Data: data, Cache: filepath.Join(data, "cache")}, nil
}

// foldNames reports whether the host compares file names without case. It
// decides by operating system (windows, darwin), not by the volume, so a
// case-sensitive volume is still treated as folding; the Unicode folding of
// EqualFold is accepted because both only make a refusal more likely (fail closed).
func foldNames() bool { return runtime.GOOS == "windows" || runtime.GOOS == "darwin" }

// NamesEqual compares two file names the way the host does: without case on
// Windows and macOS, exactly elsewhere.
func NamesEqual(a, b string) bool {
	if foldNames() {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// RootOf returns the project root of a registered file: path must be
// <root>/.truss/flows/<file> and <root>/truss.yaml must exist. None of the
// project file, .truss, .truss/flows or the file may be a link.
func RootOf(path string) (string, bool) {
	if !filepath.IsAbs(path) {
		return "", false
	}
	flows := filepath.Dir(path)
	truss := filepath.Dir(flows)
	root := filepath.Dir(truss)
	if !NamesEqual(filepath.Base(flows), flowsDir) || !NamesEqual(filepath.Base(truss), trussDir) || root == truss {
		return "", false
	}
	for _, d := range []string{truss, flows} {
		if ok, err := realDirState(d); err != nil || !ok {
			return "", false
		}
	}
	for _, f := range []string{path, filepath.Join(root, FileName)} {
		if info, err := os.Lstat(f); err != nil || !info.Mode().IsRegular() {
			return "", false
		}
	}
	return root, true
}

var envName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// parseEnv reads the project-wide env: names to single values, with the
// rules of a flow file's env (spec 026). Errors name the variable, never a
// value.
func parseEnv(n *yaml.Node) (map[string]string, error) {
	if n.Kind != yaml.MappingNode {
		return nil, errors.New("env must be a mapping of names to values")
	}
	env := make(map[string]string, len(n.Content)/2)
	for i := 0; i+1 < len(n.Content); i += 2 {
		name, v := n.Content[i].Value, n.Content[i+1]
		switch {
		case !envName.MatchString(name):
			return nil, fmt.Errorf("env: %q is not a valid variable name", name)
		case pipeline.Reserved(name):
			return nil, fmt.Errorf("env: %s is set by TRUSS in every run and cannot be set here", name)
		case v.Kind != yaml.ScalarNode:
			return nil, fmt.Errorf("env: value of %s must be a single value", name)
		case v.Tag == "!!null":
			env[name] = ""
		default:
			if err := pipeline.CheckTemplate(v.Value); err != nil {
				return nil, fmt.Errorf("env: value of %s: %w", name, err)
			}
			env[name] = v.Value
		}
	}
	return env, nil
}
