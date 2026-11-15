package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/afri-bit/truss/internal/pipeline"
	"github.com/afri-bit/truss/internal/project"
)

// pipelineFile is a pipeline file chosen by a command argument (spec 016).
type pipelineFile struct {
	path    string // absolute path
	display string // how messages name it: .truss/flows/<file>, or the path as given
	stem    string // the flow name: the file name without its extension
}

// workingRoot is the project root: the working directory, made absolute.
func workingRoot(cwd string) string {
	if abs, err := filepath.Abs(cwd); err == nil {
		return abs
	}
	return cwd
}

// isProjectFile reports whether arg is a path to the root truss.yaml.
func isProjectFile(cwd, arg string) bool {
	if !project.IsPath(arg) {
		return false
	}
	return filepath.Clean(absPath(cwd, arg)) == filepath.Join(workingRoot(cwd), project.FileName)
}

func absPath(cwd, arg string) string {
	if filepath.IsAbs(arg) {
		return arg
	}
	return filepath.Join(workingRoot(cwd), arg)
}

// resolveFile turns a run, validate or register argument into a pipeline
// file: a path when it holds a slash or ends in .yaml or .yml, else a flow
// name in the project. TRUSS never tries the other reading.
func resolveFile(cwd, arg string) (pipelineFile, error) {
	if project.IsPath(arg) {
		if isProjectFile(cwd, arg) {
			return pipelineFile{}, errors.New("truss.yaml is project metadata, not a pipeline; pipelines live in .truss/flows/")
		}
		return pipelineFile{path: absPath(cwd, arg), display: arg, stem: project.FileStem(arg)}, nil
	}
	proj, err := project.Open(workingRoot(cwd))
	if err != nil {
		return pipelineFile{}, err
	}
	flow, err := proj.Resolve(arg)
	if err != nil {
		return pipelineFile{}, err
	}
	return pipelineFile{path: flow.Path, display: ".truss/flows/" + flow.File, stem: flow.Name}, nil
}

// needFlowError is the error of a command that wants a pipeline and got none.
func needFlowError(cwd, what string) error {
	proj, err := project.Open(workingRoot(cwd))
	if err != nil {
		return err
	}
	flows, err := proj.Flows()
	if err != nil {
		return err
	}
	return fmt.Errorf("%s needs a pipeline name or path; %s", what, project.AvailableText(project.Names(flows)))
}

// loaded is a parsed and validated pipeline.
type loaded struct {
	def pipeline.Pipeline
	// declared is the name written in the file, empty when it has none.
	declared string
}

// load reads and validates a pipeline file. A pipeline without a name takes
// the file stem (spec 016 FR-008).
func load(ctx context.Context, f pipelineFile) (loaded, error) {
	def, err := pipeline.ParseFile(ctx, f.path)
	if err != nil {
		return loaded{}, err
	}
	l := loaded{def: def, declared: def.Name}
	if l.def.Name == "" {
		l.def.Name = f.stem
	}
	if err := pipeline.Validate(l.def); err != nil {
		return loaded{}, err
	}
	return l, nil
}

// prepareState makes the state folders of one pipeline under root, each with
// mode 0700, and returns their paths. No link is followed. The first time
// state is made it also writes .truss/.gitignore (FR-013 to FR-015).
func prepareState(root, stem string, stderr io.Writer) (project.State, error) {
	st, err := project.StateFor(root, stem)
	if err != nil {
		return st, err
	}
	// .truss keeps the 0755 (less umask) it always had.
	if _, err := ensureDir(st.Truss, 0o755); err != nil {
		return st, err
	}
	created, err := ensureDir(st.StateDir, 0o700)
	if err != nil {
		return st, err
	}
	if created {
		writeGitignore(filepath.Join(st.Truss, ".gitignore"), stderr)
	}
	for _, d := range []string{st.Dir, st.Data, st.Cache} {
		if _, err := ensureDir(d, 0o700); err != nil {
			return st, err
		}
	}
	return st, nil
}

// writeGitignore creates the file with state/ in it, only when it does not
// exist (O_EXCL: an existing file, link or dangling link is never written).
// A failure is a warning; the run goes on.
func writeGitignore(path string, stderr io.Writer) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644) // #nosec G302 G304 -- new file under .truss; a plain text file for git
	if errors.Is(err, fs.ErrExist) {
		return
	}
	if err == nil {
		_, err = f.WriteString("state/\n")
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}
	if err != nil {
		fmt.Fprintf(stderr, "warning: could not write %s: %s\n", filepath.ToSlash(filepath.Join(".truss", filepath.Base(path))), err.Error())
	}
}
