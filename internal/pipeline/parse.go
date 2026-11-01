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
			if val.Kind != yaml.MappingNode {
				continue
			}
			for j := 0; j < len(val.Content); j += 2 {
				field, v := val.Content[j].Value, val.Content[j+1]
				switch field {
				case "name":
					if v.Kind == yaml.ScalarNode {
						p.Name = v.Value
					}
				case "concurrency":
					parseConcurrency(&p, v)
				default:
					p.noteUnknown(field, "in pipeline")
				}
			}
		case "steps":
			if val.Kind != yaml.SequenceNode {
				p.stepsNotList = true
				continue
			}
			for pos, item := range val.Content {
				if item.Kind != yaml.MappingNode {
					p.Steps = append(p.Steps, Step{})
					continue
				}
				var step Step
				var unknown string
				for j := 0; j < len(item.Content); j += 2 {
					switch item.Content[j].Value {
					case "id":
						if item.Content[j+1].Kind == yaml.ScalarNode {
							step.ID = strings.TrimSpace(item.Content[j+1].Value)
						}
					case "run":
						if item.Content[j+1].Kind == yaml.ScalarNode && item.Content[j+1].Tag == "!!str" {
							step.Run = item.Content[j+1].Value
						} else {
							step.runNotString = true
						}
					case "needs":
						parseNeeds(&step, item.Content[j+1])
					default:
						if unknown == "" {
							unknown = item.Content[j].Value
						}
					}
				}
				if unknown != "" {
					where := fmt.Sprintf("in step %d", pos+1)
					if step.ID != "" {
						where = fmt.Sprintf("in step %q", step.ID)
					}
					p.noteUnknown(unknown, where)
				}
				p.Steps = append(p.Steps, step)
			}
		default:
			p.noteUnknown(key, "at top level")
		}
	}
	p.Name = strings.TrimSpace(p.Name)
	return p, nil
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
