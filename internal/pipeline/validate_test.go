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

// FR-016 replaces the old rule that ignored unknown fields: depends_on is not
// a dependency and is now rejected as an unknown field.
func TestValidate_DependsOnIsNotNeeds(t *testing.T) {
	const doc = "version: \"1\"\npipeline:\n  name: n\nsteps:\n" +
		"  - id: a\n    depends_on: [ghost]\n    run: echo a\n" +
		"  - id: b\n    depends_on: b\n    run: echo b\n"
	p, err := Parse([]byte(doc))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	err = Validate(p)
	if want := `unknown field "depends_on" in step "a"`; err == nil || err.Error() != want {
		t.Fatalf("Validate() error = %v, want %q", err, want)
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

func validateDoc(t *testing.T, doc string) error {
	t.Helper()
	p, err := Parse([]byte(doc))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	return Validate(p)
}

func TestValidate_RejectsInvalidConcurrency(t *testing.T) {
	const steps = "steps:\n  - id: a\n    run: echo a\n"
	// For an empty value the diagnostic shows "" and for null or ~ it shows
	// "null"; the contract only says "<value>", this is the chosen reading.
	scalars := []struct{ raw, shown string }{
		{"0", "0"}, {"-1", "-1"}, {"1.5", "1.5"}, {"1.0", "1.0"},
		{`"two"`, "two"}, {`"3"`, "3"}, {"true", "true"},
		{"", ""}, {"null", "null"}, {"~", "null"},
		{"0x10", "0x10"}, {"0o7", "0o7"}, {"1_000", "1_000"}, {"+3", "+3"},
	}
	for _, tt := range scalars {
		t.Run("value "+tt.raw, func(t *testing.T) {
			doc := "version: \"1\"\npipeline:\n  name: n\n  concurrency: " + tt.raw + "\n" + steps
			want := `pipeline concurrency must be a whole number of at least 1, got "` + tt.shown + `"`
			if err := validateDoc(t, doc); err == nil || err.Error() != want {
				t.Fatalf("Validate() error = %v, want %q", err, want)
			}
		})
	}
	nonScalars := []struct{ raw, shown string }{{"[]", "a list"}, {"{}", "a mapping"}, {"[1, 2]", "a list"}, {"{a: 1}", "a mapping"}}
	for _, tt := range nonScalars {
		t.Run("value "+tt.raw, func(t *testing.T) {
			doc := "version: \"1\"\npipeline:\n  name: n\n  concurrency: " + tt.raw + "\n" + steps
			want := "pipeline concurrency must be a whole number of at least 1, got " + tt.shown
			if err := validateDoc(t, doc); err == nil || err.Error() != want {
				t.Fatalf("Validate() error = %v, want %q", err, want)
			}
		})
	}
}

// A negative value set from Go code (not from a file) is also rejected (T029).
func TestValidate_RejectsNegativeConcurrencyFromCode(t *testing.T) {
	p := Pipeline{Version: "1", Name: "n", Concurrency: -1, Steps: []Step{{ID: "a", Run: "echo a"}}}
	want := `pipeline concurrency must be a whole number of at least 1, got "-1"`
	if err := Validate(p); err == nil || err.Error() != want {
		t.Fatalf("Validate() error = %v, want %q", err, want)
	}
}

func TestValidate_AcceptsValidConcurrency(t *testing.T) {
	for _, n := range []string{"1", "2", "16", "99999999999999999999"} {
		t.Run(n, func(t *testing.T) {
			doc := "version: \"1\"\npipeline:\n  name: n\n  concurrency: " + n + "\nsteps:\n  - id: a\n    run: echo a\n"
			if err := validateDoc(t, doc); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestValidate_RejectsUnknownFields(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want string
	}{
		{
			name: "top level",
			doc:  "version: \"1\"\nstpes: []\npipeline:\n  name: n\nsteps:\n  - id: a\n    run: echo a\n",
			want: `unknown field "stpes" at top level`,
		},
		{
			name: "under pipeline",
			doc:  "version: \"1\"\npipeline:\n  name: n\n  concurency: 2\nsteps:\n  - id: a\n    run: echo a\n",
			want: `unknown field "concurency" in pipeline`,
		},
		{
			name: "in a step with an id",
			doc:  "version: \"1\"\npipeline:\n  name: n\nsteps:\n  - id: build\n    need: [x]\n    run: echo a\n",
			want: `unknown field "need" in step "build"`,
		},
		{
			name: "in a step without an id uses the 1-based position",
			doc:  "version: \"1\"\npipeline:\n  name: n\nsteps:\n  - id: a\n    run: echo a\n  - need: [a]\n    run: echo b\n",
			want: `unknown field "need" in step 2`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateDoc(t, tt.doc); err == nil || err.Error() != tt.want {
				t.Fatalf("Validate() error = %v, want %q", err, tt.want)
			}
		})
	}
}

// Check order: version, unknown field, name, concurrency, steps, id, run, needs.
// Each case carries the two adjacent faults; the earlier one must win.
func TestValidate_CheckOrder(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want string
	}{
		{
			name: "version before unknown field",
			doc:  "version: \"2\"\nstpes: []\npipeline:\n  name: n\nsteps:\n  - id: a\n    run: echo a\n",
			want: `unsupported schema version "2"`,
		},
		{
			name: "unknown field before name",
			doc:  "version: \"1\"\npipeline:\n  concurency: 2\nsteps:\n  - id: a\n    run: echo a\n",
			want: `unknown field "concurency" in pipeline`,
		},
		{
			name: "concurrency without a name",
			doc:  "version: \"1\"\npipeline:\n  concurrency: 0\nsteps:\n  - id: a\n    run: echo a\n",
			want: `pipeline concurrency must be a whole number of at least 1, got "0"`,
		},
		{
			name: "concurrency before steps",
			doc:  "version: \"1\"\npipeline:\n  name: n\n  concurrency: 0\nsteps: []\n",
			want: `pipeline concurrency must be a whole number of at least 1, got "0"`,
		},
		{
			name: "concurrency before a missing step id",
			doc:  "version: \"1\"\npipeline:\n  name: n\n  concurrency: 0\nsteps:\n  - run: echo a\n",
			want: `pipeline concurrency must be a whole number of at least 1, got "0"`,
		},
		{
			name: "concurrency before a missing run",
			doc:  "version: \"1\"\npipeline:\n  name: n\n  concurrency: 0\nsteps:\n  - id: a\n",
			want: `pipeline concurrency must be a whole number of at least 1, got "0"`,
		},
		{
			name: "concurrency before needs",
			doc:  "version: \"1\"\npipeline:\n  name: n\n  concurrency: 0\nsteps:\n  - id: a\n    needs: [ghost]\n    run: echo a\n",
			want: `pipeline concurrency must be a whole number of at least 1, got "0"`,
		},
		{
			name: "unknown field in a step before its missing run",
			doc:  "version: \"1\"\npipeline:\n  name: n\nsteps:\n  - id: a\n    need: [b]\n",
			want: `unknown field "need" in step "a"`,
		},
		{
			name: "unknown field in a step before its missing id",
			doc:  "version: \"1\"\npipeline:\n  name: n\nsteps:\n  - need: [b]\n    run: echo a\n",
			want: `unknown field "need" in step 1`,
		},
		{
			name: "unknown field at top level before an empty step list",
			doc:  "version: \"1\"\nstpes: []\npipeline:\n  name: n\nsteps: []\n",
			want: `unknown field "stpes" at top level`,
		},
		{
			name: "run before needs",
			doc:  "version: \"1\"\npipeline:\n  name: n\nsteps:\n  - id: a\n    needs: [ghost]\n    run: echo a\n  - id: b\n",
			want: `missing or empty run for step "b"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateDoc(t, tt.doc); err == nil || err.Error() != tt.want {
				t.Fatalf("Validate() error = %v, want %q", err, tt.want)
			}
		})
	}
}

// Story 4, scenario 3: a valid limit does not hide the existing needs errors.
func TestValidate_ValidConcurrencyWithBadGraphReportsNeedsError(t *testing.T) {
	tests := []struct{ name, steps, want string }{
		{"unknown need", "  - id: a\n    needs: [ghost]\n    run: echo a\n", `step "a" needs unknown step "ghost"`},
		{"cycle", "  - id: a\n    needs: [b]\n    run: echo a\n  - id: b\n    needs: [a]\n    run: echo b\n", "dependency cycle: a, b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := "version: \"1\"\npipeline:\n  name: n\n  concurrency: 4\nsteps:\n" + tt.steps
			if err := validateDoc(t, doc); err == nil || err.Error() != tt.want {
				t.Fatalf("Validate() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestValidate_AcceptsOnlyRecognisedFields(t *testing.T) {
	const doc = "version: \"1\"\n" +
		"pipeline:\n  name: build\n  concurrency: 2\n" +
		"steps:\n" +
		"  - id: lint\n    run: echo lint\n" +
		"  - id: test\n    run: echo test\n" +
		"  - id: package\n    needs: [lint, test]\n    run: echo package\n"
	if err := validateDoc(t, doc); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

// Spec 016 FR-008: pipeline.name is optional; callers default it to the file stem.
func TestValidate_NameIsOptional(t *testing.T) {
	p, err := Parse([]byte("version: \"1\"\nsteps:\n  - id: a\n    run: echo a\n"))
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(p); err != nil {
		t.Fatalf("Validate() = %v, want no error for a pipeline without a name", err)
	}
}
