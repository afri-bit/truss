package pipeline

import (
	"strings"
	"testing"
)

func evalWith(t *testing.T, text string, success, failure bool, env map[string]string) bool {
	t.Helper()
	c, err := ParseCondition(text)
	if err != nil {
		t.Fatalf("ParseCondition(%q) = %v", text, err)
	}
	return c.Eval(EvalContext{Success: success, Failure: failure, Env: func(n string) string { return env[n] }})
}

func TestCondition_StatusFunctions(t *testing.T) {
	tests := []struct {
		text             string
		success, failure bool
		want             bool
	}{
		{"always()", false, false, true},
		{"always()", false, true, true},
		{"success()", true, false, true},
		{"success()", false, true, false},
		{"success()", false, false, false}, // a needed step was skipped
		{"failure()", false, true, true},
		{"failure()", true, false, false},
		{"failure() || success()", false, false, false},
		{"!failure()", true, false, true},
		{"failure() && env.X == 'a'", false, true, false}, // X is unset
	}
	for _, tt := range tests {
		if got := evalWith(t, tt.text, tt.success, tt.failure, nil); got != tt.want {
			t.Errorf("%q with success=%v failure=%v = %v, want %v", tt.text, tt.success, tt.failure, got, tt.want)
		}
	}
}

func TestCondition_EnvComparisons(t *testing.T) {
	env := map[string]string{"BRANCH": "main", "EMPTY": "", "N": "3"}
	tests := []struct {
		text string
		want bool
	}{
		{`env.BRANCH == 'main'`, true},
		{`env.BRANCH == "main"`, true},
		{`env.BRANCH != 'main'`, false},
		{`env.BRANCH != 'dev'`, true},
		{`env.EMPTY == ''`, true},
		{`env.UNSET == ''`, true},
		{`'main' == env.BRANCH`, true},
		{`env.BRANCH == env.BRANCH`, true},
		{`env.N == '3'`, true},
		{`env.BRANCH == 'main' && env.N != '4'`, true},
		{`env.BRANCH == 'dev' || env.N == '3'`, true},
		{`!(env.BRANCH == 'dev')`, true},
		{`env.BRANCH == 'a || b'`, false}, // operators inside text are text
		{`  env.BRANCH   ==   'main'  `, true},
	}
	for _, tt := range tests {
		if got := evalWith(t, tt.text, true, false, env); got != tt.want {
			t.Errorf("%q = %v, want %v", tt.text, got, tt.want)
		}
	}
}

func TestCondition_WithoutAStatusFunctionImpliesSuccess(t *testing.T) {
	if evalWith(t, `env.A == ''`, false, true, nil) {
		t.Error("a comparison held although a needed step failed")
	}
	if !evalWith(t, `always() && env.A == ''`, false, true, nil) {
		t.Error("always() should lift the implied success()")
	}
}

func TestCondition_Precedence(t *testing.T) {
	// && binds tighter than ||, and ! tighter than both.
	if !evalWith(t, `always() || failure() && !always()`, true, false, nil) {
		t.Error("|| and && precedence")
	}
	if evalWith(t, `(always() || failure()) && !always()`, true, false, nil) {
		t.Error("parentheses")
	}
}

func TestCondition_AfterFailure(t *testing.T) {
	tests := map[string]bool{
		"always()":              true,
		"failure()":             true,
		"success()":             false,
		"env.A == 'b'":          false,
		"success() || always()": true,
	}
	for text, want := range tests {
		c, err := ParseCondition(text)
		if err != nil {
			t.Fatal(err)
		}
		if c.AfterFailure() != want {
			t.Errorf("%q AfterFailure = %v, want %v", text, c.AfterFailure(), want)
		}
	}
	var none *Condition
	if none.AfterFailure() || !none.Eval(EvalContext{}) || none.String() != "" {
		t.Error("a nil condition holds and never runs after a failure")
	}
}

func TestParseCondition_RejectsBadExpressions(t *testing.T) {
	tests := []struct{ text, want string }{
		{"", "empty"},
		{"   ", "empty"},
		{"always(", "takes no arguments"},
		{"always(1)", "takes no arguments"},
		{"sometimes()", `unknown function "sometimes"`},
		{"env.A", "expected == or !="},
		{"env.A ==", "expected a value"},
		{"env.A == 'b", "not closed"},
		{"env.A == b", "not a value"},
		{"env. == 'x'", "not a value"},
		{"env.1A == 'x'", "not a value"},
		{"env.A = 'b'", "unexpected character"},
		{"always() always()", "unexpected"},
		{"(always()", `expected ")"`},
		{"always())", "unexpected"},
		{"always() &&", "expected a value"},
		{"&& always()", "expected a value"},
		{"failure() | always()", "unexpected character"},
		{"1 == 1", "not a value"},
		{"$HOME == 'x'", "unexpected character"},
		{strings.Repeat("(", 40) + "always()" + strings.Repeat(")", 40), "nested too deeply"},
		{strings.Repeat("always() && ", 60) + "always()", "longer than"},
	}
	for _, tt := range tests {
		_, err := ParseCondition(tt.text)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("ParseCondition(%q) error = %v, want it to contain %q", tt.text, err, tt.want)
		}
	}
}
