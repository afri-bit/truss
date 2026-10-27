package pipeline

import (
	"strings"
	"testing"
)

func TestValidate_RejectsInvalidNeeds(t *testing.T) {
	const head = "version: \"1\"\npipeline:\n  name: n\nsteps:\n"
	tests := []struct {
		name  string
		steps string
		want  string
	}{
		{
			name:  "scalar needs",
			steps: "  - id: a\n    needs: build\n    run: echo a\n",
			want:  `step "a" needs must be a list of step ids`,
		},
		{
			name:  "number entry",
			steps: "  - id: a\n    needs: [1]\n    run: echo a\n",
			want:  `step "a" needs must be a list of step ids`,
		},
		{
			name:  "null needs",
			steps: "  - id: a\n    needs:\n    run: echo a\n",
			want:  `step "a" needs must be a list of step ids`,
		},
		{
			name:  "unknown id",
			steps: "  - id: a\n    needs: [ghost]\n    run: echo a\n",
			want:  `step "a" needs unknown step "ghost"`,
		},
		{
			name:  "empty entry",
			steps: "  - id: a\n    needs: [\"\"]\n    run: echo a\n",
			want:  `step "a" needs unknown step ""`,
		},
		{
			name:  "ids are compared exactly",
			steps: "  - id: build\n    run: echo build\n  - id: test\n    needs: [Build]\n    run: echo test\n",
			want:  `step "test" needs unknown step "Build"`,
		},
		{
			name:  "self dependency",
			steps: "  - id: a\n    needs: [a]\n    run: echo a\n",
			want:  `step "a" depends on itself`,
		},
		{
			name:  "repeated id",
			steps: "  - id: a\n    run: echo a\n  - id: b\n    needs: [a, a]\n    run: echo b\n",
			want:  `step "b" repeats step "a"`,
		},
		{
			name:  "two step cycle",
			steps: "  - id: a\n    needs: [b]\n    run: echo a\n  - id: b\n    needs: [a]\n    run: echo b\n",
			want:  "dependency cycle: a, b",
		},
		{
			name:  "three step cycle",
			steps: "  - id: a\n    needs: [b]\n    run: echo a\n  - id: b\n    needs: [c]\n    run: echo b\n  - id: c\n    needs: [a]\n    run: echo c\n",
			want:  "dependency cycle: a, b, c",
		},
		{
			name:  "cycle starts at the member written earliest",
			steps: "  - id: c\n    needs: [a]\n    run: echo c\n  - id: a\n    needs: [b]\n    run: echo a\n  - id: b\n    needs: [c]\n    run: echo b\n",
			want:  "dependency cycle: c, a, b",
		},
		{
			name:  "cycle reached from a step outside it names only the cycle",
			steps: "  - id: x\n    needs: [a]\n    run: echo x\n  - id: a\n    needs: [b]\n    run: echo a\n  - id: b\n    needs: [a]\n    run: echo b\n",
			want:  "dependency cycle: a, b",
		},
		{
			name:  "first fault in definition order wins",
			steps: "  - id: a\n    needs: [ghost]\n    run: echo a\n  - id: b\n    needs: [b]\n    run: echo b\n",
			want:  `step "a" needs unknown step "ghost"`,
		},
		{
			name:  "list faults are reported before a cycle",
			steps: "  - id: a\n    needs: [b]\n    run: echo a\n  - id: b\n    needs: [a, ghost]\n    run: echo b\n",
			want:  `step "b" needs unknown step "ghost"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := Parse([]byte(head + tt.steps))
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			err = Validate(p)
			if err == nil || err.Error() != tt.want {
				t.Fatalf("Validate() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestValidate_AcceptsValidNeeds(t *testing.T) {
	const doc = "version: \"1\"\npipeline:\n  name: n\nsteps:\n" +
		"  - id: package\n    needs: [lint, test]\n    run: echo package\n" +
		"  - id: lint\n    run: echo lint\n" +
		"  - id: test\n    needs: [lint]\n    run: echo test\n" +
		"  - id: scan\n    needs: []\n    run: echo scan\n"
	p, err := Parse([]byte(doc))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if err := Validate(p); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidate_StructuralErrorsComeBeforeNeedsErrors(t *testing.T) {
	const head = "version: \"1\"\npipeline:\n  name: n\nsteps:\n"
	tests := []struct {
		name  string
		steps string
		want  string
	}{
		{
			name:  "later step has an empty run",
			steps: "  - id: a\n    needs: [ghost]\n    run: echo a\n  - id: b\n    run: \"\"\n",
			want:  `missing or empty run for step "b"`,
		},
		{
			name:  "later step repeats an id",
			steps: "  - id: a\n    needs: [a]\n    run: echo a\n  - id: a\n    run: echo b\n",
			want:  `duplicate step id "a"`,
		},
		{
			name:  "later step has no id",
			steps: "  - id: a\n    needs: build\n    run: echo a\n  - run: echo b\n",
			want:  "missing step id",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := Parse([]byte(head + tt.steps))
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			err = Validate(p)
			if err == nil || err.Error() != tt.want {
				t.Fatalf("Validate() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestValidate_DependsOnIsNotNeeds(t *testing.T) {
	const doc = "version: \"1\"\npipeline:\n  name: n\nsteps:\n" +
		"  - id: a\n    depends_on: [ghost]\n    run: echo a\n" +
		"  - id: b\n    depends_on: b\n    run: echo b\n"
	p, err := Parse([]byte(doc))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if err := Validate(p); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidate_ExistingMessagesAreUnchanged(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want string
	}{
		{name: "missing version", yaml: "pipeline:\n  name: n\nsteps:\n  - id: a\n    run: echo a\n", want: "missing or unsupported schema version"},
		{name: "unsupported version", yaml: "version: \"2\"\npipeline:\n  name: n\nsteps:\n  - id: a\n    run: echo a\n", want: `unsupported schema version "2"`},
		{name: "missing name", yaml: "version: \"1\"\nsteps:\n  - id: a\n    run: echo a\n", want: "missing pipeline name"},
		{name: "empty steps", yaml: "version: \"1\"\npipeline:\n  name: n\nsteps: []\n", want: "pipeline must contain one or more steps"},
		{name: "duplicate id", yaml: "version: \"1\"\npipeline:\n  name: n\nsteps:\n  - id: a\n    run: echo a\n  - id: a\n    run: echo b\n", want: `duplicate step id "a"`},
		{name: "empty run", yaml: "version: \"1\"\npipeline:\n  name: n\nsteps:\n  - id: a\n    run: \"\"\n", want: `missing or empty run for step "a"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := Parse([]byte(tt.yaml))
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			err = Validate(p)
			if err == nil || err.Error() != tt.want {
				t.Fatalf("Validate() error = %v, want %q", err, tt.want)
			}
		})
	}
}

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
