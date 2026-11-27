package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/afri-bit/truss/internal/cron"
	"github.com/afri-bit/truss/internal/pipeline"
	"github.com/afri-bit/truss/internal/project"
	"github.com/afri-bit/truss/internal/registry"
)

// A template is the steps of a first flow for one kind of project (spec 031).
type template struct {
	name, about string
	detect      []string // files that suggest it
	steps       string   // the steps: block, indented as in a flow file
}

var templates = []template{
	{"go", "Go module: vet, test and build", []string{"go.mod"},
		"steps:\n  - id: vet\n    run: go vet ./...\n  - id: test\n    run: go test ./...\n    needs: [vet]\n  - id: build\n    run: go build ./...\n    needs: [test]\n"},
	{"node", "Node.js package: install and test", []string{"package.json"},
		"steps:\n  - id: install\n    run: npm ci\n  - id: test\n    run: npm test\n    needs: [install]\n"},
	{"python", "Python project: install and test", []string{"pyproject.toml", "requirements.txt", "setup.py"},
		"steps:\n  - id: install\n    run: pip install -r requirements.txt\n  - id: test\n    run: python -m pytest\n    needs: [install]\n"},
	{"make", "Makefile: make and make test", []string{"Makefile"},
		"steps:\n  - id: build\n    run: make\n  - id: test\n    run: make test\n    needs: [build]\n"},
	{"backup", "Copy a folder somewhere safe", nil, backupSteps()},
	{"shell", "One step that says hello: a start to edit", nil,
		"steps:\n  - id: hello\n    run: echo \"Hello from TRUSS\"\n"},
}

// backupSteps copies a folder with the host shell's own command.
func backupSteps() string {
	run := `cp -R ./important "$TRUSS_DATA_DIR/backup"`
	if runtime.GOOS == "windows" {
		run = `xcopy /E /I /Y important "%TRUSS_DATA_DIR%\backup"`
	}
	return "steps:\n  - id: copy\n    # Change both folders. TRUSS_DATA_DIR is a folder that stays between runs.\n    run: '" +
		strings.ReplaceAll(run, "'", "''") + "'\n"
}

func findTemplate(name string) (template, bool) {
	for _, t := range templates {
		if t.name == name {
			return t, true
		}
	}
	return template{}, false
}

func templateNames() string {
	names := make([]string, len(templates))
	for i, t := range templates {
		names[i] = t.name
	}
	return strings.Join(names, ", ")
}

// detectTemplate picks the template whose marker file is in dir.
func detectTemplate(dir string) template {
	for _, t := range templates {
		for _, f := range t.detect {
			if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
				return t
			}
		}
	}
	t, _ := findTemplate("shell")
	return t
}

// flowAnswers are what a new flow needs.
type flowAnswers struct {
	name     string
	tmpl     template
	start    string // "hand", "schedule" or "files"
	cron     string
	paths    string
	notifyOn bool
}

// asker reads answers from a terminal, or takes the defaults.
type asker struct {
	in  *bufio.Reader
	out io.Writer
	yes bool
}

// ask prints a question with its default and returns the answer.
func (a *asker) ask(question, def string) string {
	if a.yes {
		return def
	}
	fmt.Fprintf(a.out, "%s [%s]: ", question, def)
	line, err := a.in.ReadString('\n')
	line = strings.TrimSpace(line)
	if err != nil && line == "" || line == "" {
		return def
	}
	return line
}

// choose asks for one of the options, by number or name.
func (a *asker) choose(question string, options []string, def int) int {
	if a.yes {
		return def
	}
	fmt.Fprintln(a.out, question)
	for i, o := range options {
		fmt.Fprintf(a.out, "  %d) %s\n", i+1, o)
	}
	for {
		answer := a.ask("Choose", fmt.Sprint(def+1))
		for i, o := range options {
			if answer == fmt.Sprint(i+1) || strings.HasPrefix(o, answer+" ") || o == answer {
				return i
			}
		}
		fmt.Fprintf(a.out, "Please answer 1 to %d.\n", len(options))
	}
}

// schedules offers schedules in plain words; the last one asks for cron.
var schedules = []struct{ words, cron string }{
	{"every hour", "0 * * * *"},
	{"every day at 02:00", "0 2 * * *"},
	{"weekdays at 09:00", "0 9 * * 1-5"},
	{"every Monday at 07:00", "0 7 * * 1"},
	{"something else (a cron expression)", ""},
}

// askFlow asks what a new flow is and how it starts.
func (a *asker) askFlow(dir, name string, tmplFlag string) (flowAnswers, error) {
	ans := flowAnswers{name: a.ask("Name of the flow", name), start: "hand"}
	if !validFlowName(ans.name) {
		return ans, fmt.Errorf("flow name %q: use letters, digits, '.', '_' and '-', starting with a letter or digit", ans.name)
	}
	detected := detectTemplate(dir)
	if tmplFlag != "" {
		t, ok := findTemplate(tmplFlag)
		if !ok {
			return ans, fmt.Errorf("no template %q; there are: %s", tmplFlag, templateNames())
		}
		ans.tmpl = t
	} else {
		options := make([]string, len(templates))
		def := 0
		for i, t := range templates {
			options[i] = t.name + " - " + t.about
			if t.name == detected.name {
				def = i
				for _, f := range t.detect {
					if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
						options[i] += " (found " + f + ")"
						break
					}
				}
			}
		}
		ans.tmpl = templates[a.choose("What should the flow do?", options, def)]
	}
	switch a.choose("How should it start?", []string{
		"by hand (truss run " + ans.name + ")", "on a schedule", "when files change"}, 0) {
	case 1:
		ans.start = "schedule"
		words := make([]string, len(schedules))
		for i, s := range schedules {
			words[i] = s.words
		}
		i := a.choose("When?", words, 1)
		ans.cron = schedules[i].cron
		for ans.cron == "" {
			c := a.ask("Cron expression (minute hour day month weekday)", "0 2 * * *")
			if _, err := cron.Parse(c); err != nil {
				fmt.Fprintf(a.out, "%s\n", err.Error())
				continue
			}
			ans.cron = c
		}
	case 2:
		ans.start = "files"
		ans.paths = a.ask("Which folder or pattern?", "src/")
		if err := pipeline.CheckPattern(ans.paths); err != nil {
			return ans, err
		}
	}
	ans.notifyOn = strings.HasPrefix(strings.ToLower(a.ask("Show a desktop notification when it fails? (y/n)", "n")), "y")
	return ans, nil
}

var flowNameText = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

func validFlowName(name string) bool { return flowNameText.MatchString(name) }

// flowFile writes the YAML of a new flow, with comments that say what to
// change next.
func flowFile(ans flowAnswers) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# A TRUSS flow. Reference: https://github.com/afri-bit/truss/blob/main/docs/pipeline-reference.md\n")
	b.WriteString("version: \"1\"\n\nflow:\n  name: " + ans.name + "\n")
	switch ans.start {
	case "schedule":
		b.WriteString("\n# Runs while the daemon runs: truss register --all && truss daemon start\non:\n  schedule:\n    - cron: \"" + ans.cron + "\"\n")
	case "files":
		b.WriteString("\n# Runs while the daemon runs, once changes stop for 2s. The step reads\n# the changed paths from the file named by $TRUSS_CHANGED_FILES.\non:\n  file:\n    paths: [" + yamlQuote(ans.paths) + "]\n")
	}
	if ans.notifyOn {
		b.WriteString("\n# Tells you when a run fails. Add webhook: or run: for Slack, email and more.\nnotify:\n  on: failure\n  desktop: true\n")
	}
	b.WriteString("\n" + ans.tmpl.steps)
	return b.String()
}

func yamlQuote(s string) string {
	return "\"" + strings.ReplaceAll(strings.ReplaceAll(s, "\\", "\\\\"), "\"", "\\\"") + "\""
}

// writeNew writes a file that must not exist yet.
func writeNew(path, text string, perm os.FileMode) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, perm) // #nosec G304 -- a path in the project the user asked for
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("%s exists already; TRUSS does not overwrite it", filepath.Base(path))
		}
		return err
	}
	if _, err := f.WriteString(text); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// initProject is truss init (spec 031): a truss.yaml and a first flow.
func (in invocation) initProject(ctx context.Context) int {
	pos, flags, err := options(in.args, []string{"name", "template", "flow"}, []string{"yes"})
	if err != nil || len(pos) != 0 {
		return in.usage()
	}
	root := workingRoot(in.cwd)
	if _, err := os.Stat(filepath.Join(root, project.FileName)); err == nil {
		return in.fail(errors.New("this folder is a TRUSS project already (truss.yaml exists); add a flow with: truss flows new <name>"))
	}
	a := in.newAsker(flags)
	if !a.yes {
		fmt.Fprintln(in.stdout, "Set up TRUSS in this folder. Press Enter to take the answer in [brackets].")
	}
	name := a.ask("Name of the project", cmpOr(flags["name"], safeName(filepath.Base(root))))
	if registry.CheckName(name) != nil {
		return in.fail(fmt.Errorf("project name %q: use letters, digits, '.', '_' and '-', starting with a letter or digit", name))
	}
	ans, err := a.askFlow(root, cmpOr(flags["flow"], defaultFlowName(detectTemplate(root))), flags["template"])
	if err != nil {
		return in.fail(err)
	}
	// The project folders are as readable as the rest of the project (spec 016).
	if err := os.MkdirAll(filepath.Join(root, ".truss", "flows"), 0o755); err != nil { // #nosec G301 -- project files, not state
		return in.fail(err)
	}
	projectText := "# The TRUSS project. Flows are files in .truss/flows/.\nversion: \"1\"\n\nproject:\n  name: " + name + "\n"
	if err := writeNew(filepath.Join(root, project.FileName), projectText, 0o644); err != nil {
		return in.fail(err)
	}
	return in.finishFlow(ctx, root, ans, true)
}

// flowsNew is truss flows new <name>: another flow in this project.
func (in invocation) flowsNew(ctx context.Context, args []string) int {
	pos, flags, err := options(args, []string{"template"}, []string{"yes"})
	if err != nil || len(pos) > 1 {
		return in.usage()
	}
	root := workingRoot(in.cwd)
	if _, err := project.Open(root); err != nil {
		return in.fail(err)
	}
	name := ""
	if len(pos) == 1 {
		name = pos[0]
	}
	a := in.newAsker(flags)
	ans, err := a.askFlow(root, cmpOr(name, "flow"), flags["template"])
	if err != nil {
		return in.fail(err)
	}
	return in.finishFlow(ctx, root, ans, false)
}

func (in invocation) newAsker(flags map[string]string) *asker {
	a := &asker{out: in.stdout, yes: flags["yes"] != ""}
	f, ok := in.stdin.(*os.File)
	if !ok || !isTerminal(f) {
		a.yes = true // no one to ask: take every default
	}
	if !a.yes {
		a.in = bufio.NewReader(f)
	}
	return a
}

// finishFlow writes the flow, checks it and says what to do next.
func (in invocation) finishFlow(ctx context.Context, root string, ans flowAnswers, newProject bool) int {
	path := filepath.Join(root, ".truss", "flows", ans.name+".yaml")
	if err := writeNew(path, flowFile(ans), 0o644); err != nil {
		return in.fail(err)
	}
	if _, err := load(ctx, pipelineFile{path: path, display: ".truss/flows/" + ans.name + ".yaml", stem: ans.name}); err != nil {
		return in.fail(fmt.Errorf("the new flow does not load (please report this): %w", err))
	}
	if newProject {
		fmt.Fprintf(in.stdout, "created truss.yaml\n")
	}
	fmt.Fprintf(in.stdout, "created .truss/flows/%s.yaml (%s)\n\nNext:\n", ans.name, ans.tmpl.about)
	fmt.Fprintf(in.stdout, "  truss run %s                    run it now, here\n", ans.name)
	fmt.Fprintf(in.stdout, "  truss flows show %s             see its steps\n", ans.name)
	if ans.start != "hand" || ans.notifyOn {
		fmt.Fprintln(in.stdout, "  truss register --all && truss daemon start")
		fmt.Fprintln(in.stdout, "                                   let the daemon start it on its own")
	}
	return 0
}

func defaultFlowName(t template) string {
	switch t.name {
	case "backup":
		return "backup"
	case "shell":
		return "hello"
	}
	return "build"
}

// safeName turns a folder name into a project name.
func safeName(s string) string {
	s = regexp.MustCompile(`[^A-Za-z0-9._-]+`).ReplaceAllString(s, "-")
	s = strings.TrimLeft(s, "._-")
	if s == "" {
		return "project"
	}
	return s
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
