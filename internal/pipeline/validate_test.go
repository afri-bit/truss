package pipeline

import (
	"strings"
	"testing"
)

func TestValidate_RejectsInvalidDefinitions(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want string
	}{
		{name: "missing name", yaml: "version: \"1\"\nsteps:\n  - id: a\n    run: echo a\n", want: "name"},
		{name: "empty steps", yaml: "version: \"1\"\npipeline:\n  name: n\nsteps: []\n", want: "steps"},
		{name: "duplicate id", yaml: "version: \"1\"\npipeline:\n  name: n\nsteps:\n  - id: a\n    run: echo a\n  - id: a\n    run: echo b\n", want: "duplicate"},
		{name: "empty id", yaml: "version: \"1\"\npipeline:\n  name: n\nsteps:\n  - id: \"\"\n    run: echo a\n", want: "id"},
		{name: "empty run", yaml: "version: \"1\"\npipeline:\n  name: n\nsteps:\n  - id: a\n    run: \"\"\n", want: "run"},
		{name: "bad version", yaml: "version: \"2\"\npipeline:\n  name: n\nsteps:\n  - id: a\n    run: echo a\n", want: "version"},
		{name: "numeric version", yaml: "version: 1\npipeline:\n  name: n\nsteps:\n  - id: a\n    run: echo a\n", want: "version"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := Parse([]byte(tt.yaml))
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			err = Validate(p)
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), tt.want) {
				t.Fatalf("Validate() error = %v, want %q", err, tt.want)
			}
		})
	}
}
