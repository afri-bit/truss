package pipeline

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

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
				if val.Content[j].Value == "name" && val.Content[j+1].Kind == yaml.ScalarNode {
					p.Name = val.Content[j+1].Value
				}
			}
		case "steps":
			if val.Kind != yaml.SequenceNode {
				p.stepsNotList = true
				continue
			}
			for _, item := range val.Content {
				if item.Kind != yaml.MappingNode {
					p.Steps = append(p.Steps, Step{})
					continue
				}
				var step Step
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
					}
				}
				p.Steps = append(p.Steps, step)
			}
		}
	}
	p.Name = strings.TrimSpace(p.Name)
	return p, nil
}
