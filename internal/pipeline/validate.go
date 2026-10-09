package pipeline

import (
	"fmt"
	"strings"
)

// Validate checks a parsed pipeline against schema v1. It does not execute steps.
func Validate(p Pipeline) error {
	if p.nonStringVersion || p.Version != "1" {
		if strings.TrimSpace(p.Version) == "" {
			return fmt.Errorf("missing or unsupported schema version")
		}
		return fmt.Errorf("unsupported schema version %q", p.Version)
	}
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("missing pipeline name")
	}
	if p.stepsNotList {
		return fmt.Errorf("steps must be a list")
	}
	if len(p.Steps) == 0 {
		return fmt.Errorf("pipeline must contain one or more steps")
	}
	seen := make(map[string]struct{}, len(p.Steps))
	for _, step := range p.Steps {
		id := strings.TrimSpace(step.ID)
		if id == "" {
			return fmt.Errorf("missing step id")
		}
		if _, ok := seen[id]; ok {
			return fmt.Errorf("duplicate step id %q", id)
		}
		seen[id] = struct{}{}
		if step.runNotString || strings.TrimSpace(step.Run) == "" {
			return fmt.Errorf("missing or empty run for step %q", id)
		}
	}
	return nil
}
