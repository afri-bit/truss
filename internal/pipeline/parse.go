package pipeline

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/afri-bit/truss/internal/cron"
	"gopkg.in/yaml.v3"
)

var decimalDigits = regexp.MustCompile(`^[0-9]+$`)

// ParseFile reads a pipeline definition from path. It does not execute steps.
func ParseFile(ctx context.Context, path string) (Pipeline, error) {
	if err := ctx.Err(); err != nil {
		return Pipeline{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Pipeline{}, fmt.Errorf("read pipeline definition: %w", err)
	}
	return Parse(data)
}

// Parse decodes a pipeline definition. Invalid YAML is an error.
// Schema checks are left to Validate.
func Parse(data []byte) (Pipeline, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return Pipeline{}, fmt.Errorf("invalid YAML: pipeline definition is empty")
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return Pipeline{}, fmt.Errorf("invalid YAML: %w", err)
	}
	if doc.Kind == 0 || len(doc.Content) == 0 {
		return Pipeline{}, nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return Pipeline{}, fmt.Errorf("invalid YAML: pipeline definition must be a mapping")
	}

	var p Pipeline
	for i := 0; i < len(root.Content); i += 2 {
		key := root.Content[i].Value
		val := root.Content[i+1]
		switch key {
		case "version":
			if val.Kind == yaml.ScalarNode && val.Tag == "!!str" {
				p.Version = val.Value
			} else if val.Kind == yaml.ScalarNode {
				p.nonStringVersion = true
				p.Version = val.Value
			} else {
				p.nonStringVersion = true
			}
		case "pipeline":
			parsePipelineBlock(&p, val)
		case "on":
			parseTriggers(&p, val)
		case "env":
			p.Env = parseEnv(&p, val, "env")
		case "steps":
			p.hasSteps = true
			if val.Kind != yaml.SequenceNode {
				p.stepsNotList = true
				continue
			}
			for pos, item := range val.Content {
				p.Steps = append(p.Steps, parseStep(&p, item, pos, ""))
			}
		case "jobs":
			p.hasJobs = true
			parseJobs(&p, val)
		default:
			p.noteUnknown(key, "at top level")
		}
	}
	p.Name = strings.TrimSpace(p.Name)
	return p, nil
}

func parsePipelineBlock(p *Pipeline, val *yaml.Node) {
	if val.Kind != yaml.MappingNode {
		return
	}
	for j := 0; j < len(val.Content); j += 2 {
		field, v := val.Content[j].Value, val.Content[j+1]
		switch field {
		case "name":
			if v.Kind == yaml.ScalarNode {
				p.Name = v.Value
			}
		case "concurrency":
			parseConcurrency(p, v)
		case "overlap":
			p.Overlap = Overlap(oneOf(p, v, "pipeline overlap", string(OverlapQueue), string(OverlapSkip), string(OverlapParallel)))
		default:
			p.noteUnknown(field, "in pipeline")
		}
	}
}

// parseStep reads one entry of a steps list. pos is its zero-based position
// and job is the id of the job it belongs to, or empty for a top-level step.
func parseStep(p *Pipeline, item *yaml.Node, pos int, job string) Step {
	if item.Kind != yaml.MappingNode {
		return Step{}
	}
	var step Step
	var unknown string
	var deferred []func(label string)
	for j := 0; j < len(item.Content); j += 2 {
		v := item.Content[j+1]
		switch item.Content[j].Value {
		case "id":
			if v.Kind == yaml.ScalarNode {
				step.ID = strings.TrimSpace(v.Value)
			}
		case "run":
			if v.Kind == yaml.ScalarNode && v.Tag == "!!str" {
				step.Run = v.Value
			} else {
				step.runNotString = true
			}
		case "needs":
			if job != "" {
				deferred = append(deferred, func(label string) {
					p.issue("%s cannot declare needs: the steps of a job run in order", label)
				})
				continue
			}
			parseNeeds(&step, v)
		case "env":
			deferred = append(deferred, func(label string) { step.Env = parseEnv(p, v, label+" env") })
		case "dir":
			deferred = append(deferred, func(label string) { step.Dir = parseString(p, v, label+" dir") })
		case "timeout":
			deferred = append(deferred, func(label string) { step.Timeout = parseTimeout(p, v, label) })
		default:
			if unknown == "" {
				unknown = item.Content[j].Value
			}
		}
	}
	// The label needs the id, which may be written after the other fields.
	name := strconv.Itoa(pos + 1)
	if step.ID != "" {
		name = strconv.Quote(joinID(job, step.ID))
	} else if job != "" {
		name = fmt.Sprintf("%d of job %q", pos+1, job)
	}
	for _, read := range deferred {
		read("step " + name)
	}
	if unknown != "" {
		p.noteUnknown(unknown, "in step "+name)
	}
	return step
}

// jobID is the set of characters a job id may use. "/" is left out because
// it separates the job from the step in a flattened step id.
var jobID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

func parseJobs(p *Pipeline, val *yaml.Node) {
	if val.Kind != yaml.MappingNode {
		p.issue("jobs must be a mapping of job id to job")
		return
	}
	seen := map[string]bool{}
	for i := 0; i < len(val.Content); i += 2 {
		id, body := strings.TrimSpace(val.Content[i].Value), val.Content[i+1]
		switch {
		case !jobID.MatchString(id):
			p.issue("job id %q must use only letters, digits, '.', '_' and '-'", id)
			continue
		case seen[id]:
			p.issue("duplicate job id %q", id)
			continue
		case body.Kind != yaml.MappingNode:
			p.issue("job %q must be a mapping", id)
			continue
		}
		seen[id] = true
		job := Job{ID: id}
		label := fmt.Sprintf("job %q", id)
		for j := 0; j < len(body.Content); j += 2 {
			v := body.Content[j+1]
			switch field := body.Content[j].Value; field {
			case "needs":
				var holder Step
				parseNeeds(&holder, v)
				if holder.needsInvalid {
					p.issue("%s needs must be a list of job ids", label)
				}
				job.Needs = holder.Needs
			case "env":
				job.Env = parseEnv(p, v, label+" env")
			case "dir":
				job.Dir = parseString(p, v, label+" dir")
			case "steps":
				if v.Kind != yaml.SequenceNode {
					p.issue("%s steps must be a list", label)
					continue
				}
				for pos, item := range v.Content {
					job.Steps = append(job.Steps, parseStep(p, item, pos, id))
				}
			default:
				p.noteUnknown(field, "in "+label)
			}
		}
		p.Jobs = append(p.Jobs, job)
	}
}

func parseTriggers(p *Pipeline, val *yaml.Node) {
	if val.Kind != yaml.MappingNode {
		p.issue("on must be a mapping")
		return
	}
	for i := 0; i < len(val.Content); i += 2 {
		field, v := val.Content[i].Value, val.Content[i+1]
		if field != "schedule" {
			p.noteUnknown(field, "in on")
			continue
		}
		if v.Kind != yaml.SequenceNode || len(v.Content) == 0 {
			p.issue("on.schedule must be a list with at least one schedule")
			continue
		}
		for pos, item := range v.Content {
			parseSchedule(p, item, pos+1)
		}
	}
}

func parseSchedule(p *Pipeline, item *yaml.Node, n int) {
	label := fmt.Sprintf("schedule %d", n)
	if item.Kind != yaml.MappingNode {
		p.issue("%s must be a mapping with a cron field", label)
		return
	}
	s := Schedule{Missed: MissedSkip}
	hasCron := false
	for j := 0; j < len(item.Content); j += 2 {
		v := item.Content[j+1]
		switch field := item.Content[j].Value; field {
		case "cron":
			hasCron = true
			s.Cron = strings.TrimSpace(parseString(p, v, label+" cron"))
			if _, err := cron.Parse(s.Cron); err != nil && v.Kind == yaml.ScalarNode {
				p.issue("%s: %s", label, err.Error())
			}
		case "timezone":
			s.Timezone = strings.TrimSpace(parseString(p, v, label+" timezone"))
			if _, err := time.LoadLocation(s.Timezone); err != nil || s.Timezone == "" {
				p.issue("%s: unknown timezone %q", label, s.Timezone)
			}
		case "missed":
			if m := oneOf(p, v, label+" missed", string(MissedSkip), string(MissedRun)); m != "" {
				s.Missed = Missed(m)
			}
		default:
			p.noteUnknown(field, "in "+label)
		}
	}
	if !hasCron {
		p.issue("%s is missing cron", label)
	}
	p.Schedules = append(p.Schedules, s)
}

var envName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// parseEnv reads a mapping of variable names to scalar values. Values are
// taken as written, so 1 and true become the strings "1" and "true".
func parseEnv(p *Pipeline, val *yaml.Node, label string) map[string]string {
	if val.Kind != yaml.MappingNode {
		p.issue("%s must be a mapping of names to values", label)
		return nil
	}
	env := make(map[string]string, len(val.Content)/2)
	for i := 0; i < len(val.Content); i += 2 {
		name, v := val.Content[i].Value, val.Content[i+1]
		switch {
		case !envName.MatchString(name):
			p.issue("%s: %q is not a valid variable name", label, name)
		case v.Kind != yaml.ScalarNode:
			p.issue("%s: value of %s must be a single value", label, name)
		case v.Tag == "!!null":
			env[name] = ""
		default:
			env[name] = v.Value
		}
	}
	return env
}

func parseString(p *Pipeline, val *yaml.Node, label string) string {
	if val.Kind != yaml.ScalarNode || val.Tag != "!!str" || strings.TrimSpace(val.Value) == "" {
		p.issue("%s must be a non-empty string", label)
		return ""
	}
	return val.Value
}

// oneOf reads a string that must be one of allowed. It returns "" and
// records an issue otherwise.
func oneOf(p *Pipeline, val *yaml.Node, label string, allowed ...string) string {
	if val.Kind == yaml.ScalarNode {
		for _, a := range allowed {
			if val.Value == a {
				return a
			}
		}
	}
	p.issue("%s must be one of %s, got %q", label, strings.Join(allowed, ", "), val.Value)
	return ""
}

var timeoutText = regexp.MustCompile(`^([0-9]+[hms])+$`)

// parseTimeout reads a duration written as whole hours, minutes and seconds,
// such as "90s", "5m" or "1h30m".
func parseTimeout(p *Pipeline, val *yaml.Node, label string) time.Duration {
	if val.Kind == yaml.ScalarNode && timeoutText.MatchString(val.Value) {
		if d, err := time.ParseDuration(val.Value); err == nil && d > 0 {
			return d
		}
	}
	p.issue("%s timeout must be a duration such as 90s, 5m or 1h30m, got %q", label, val.Value)
	return 0
}

func (p *Pipeline) issue(format string, args ...any) {
	p.issues = append(p.issues, fmt.Sprintf(format, args...))
}

// joinID names a step of a job. A top-level step keeps its own id.
func joinID(job, step string) string {
	if job == "" {
		return step
	}
	return job + "/" + step
}

// parseNeeds stores the step ids listed under a step's needs field. Entries
// are trimmed like step ids. A value that is not a list of strings marks the
// step so Validate can reject it; Parse itself does not fail.
func parseNeeds(step *Step, val *yaml.Node) {
	if val.Kind != yaml.SequenceNode {
		step.needsInvalid = true
		return
	}
	for _, entry := range val.Content {
		if entry.Kind != yaml.ScalarNode || entry.Tag != "!!str" {
			step.needsInvalid = true
			continue
		}
		step.Needs = append(step.Needs, strings.TrimSpace(entry.Value))
	}
}

// noteUnknown remembers the first unknown field in document order so
// Validate can reject it.
func (p *Pipeline) noteUnknown(field, where string) {
	if p.unknownField == "" {
		p.unknownField, p.unknownIn = field, where
	}
}

// parseConcurrency stores pipeline.concurrency. Only a plain decimal whole
// number of at least 1 is valid; a number too large for int is clamped to
// math.MaxInt. Any other value is remembered so Validate can reject it.
func parseConcurrency(p *Pipeline, val *yaml.Node) {
	if val.Kind == yaml.AliasNode && val.Alias != nil {
		val = val.Alias
	}
	switch val.Kind {
	case yaml.SequenceNode:
		p.concurrencyBad, p.concurrencyText = true, "a list"
		return
	case yaml.MappingNode:
		p.concurrencyBad, p.concurrencyText = true, "a mapping"
		return
	}
	// yaml.v3 may tag a huge decimal number as a float, so the text decides.
	if (val.Tag == "!!int" || val.Tag == "!!float") && decimalDigits.MatchString(val.Value) {
		n, err := strconv.ParseInt(val.Value, 10, 64)
		if err != nil {
			n = math.MaxInt
		}
		if n >= 1 {
			p.Concurrency = int(min(n, math.MaxInt))
			return
		}
	}
	shown := val.Value
	if val.Tag == "!!null" && shown != "" {
		shown = "null"
	}
	p.concurrencyBad, p.concurrencyText = true, strconv.Quote(shown)
}
