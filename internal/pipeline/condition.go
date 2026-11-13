package pipeline

import (
	"fmt"
	"strings"
)

// maxConditionLen bounds the text of one if: expression.
const maxConditionLen = 500

// Condition is a parsed if: expression. The language is small on purpose:
//
//	expr    = or
//	or      = and { "||" and }
//	and     = not { "&&" not }
//	not     = "!" not | "(" expr ")" | call | compare
//	call    = "always()" | "success()" | "failure()"
//	compare = operand ("==" | "!=") operand
//	operand = 'text' | "text" | env.NAME
//
// Text literals have no escapes. env.NAME is the value of the environment
// variable NAME the step would start with, or the empty string when it is not
// set.
//
// success() is true when every step the step needs succeeded, failure() when
// at least one of them failed, and always() is always true. An expression
// that calls none of them behaves as if it were written "success() && (...)".
type Condition struct {
	text string
	root *condNode

	usesStatus   bool
	afterFailure bool
}

// EvalContext is what a Condition is evaluated against.
type EvalContext struct {
	// Success is true when every step that the step needs succeeded.
	Success bool
	// Failure is true when at least one step that the step needs failed.
	Failure bool
	// Env returns the value of an environment variable, or "" when it is unset.
	Env func(name string) string
}

type condKind int

const (
	condCall condKind = iota
	condNot
	condAnd
	condOr
	condEq
	condNe
)

type condNode struct {
	kind condKind
	name string // the function of a condCall
	l, r *condNode
	a, b condOperand
}

type condOperand struct {
	env bool   // an env.NAME reference; otherwise a literal
	val string // the variable name or the literal text
}

// String returns the expression as it was written.
func (c *Condition) String() string {
	if c == nil {
		return ""
	}
	return c.text
}

// AfterFailure reports whether a step guarded by c may start after another
// step has failed the run: the expression calls always() or failure(). A nil
// condition returns false.
func (c *Condition) AfterFailure() bool { return c != nil && c.afterFailure }

// Eval reports whether the condition holds. A nil condition holds.
func (c *Condition) Eval(ctx EvalContext) bool {
	if c == nil {
		return true
	}
	if !c.usesStatus && !ctx.Success {
		return false
	}
	return c.root.eval(ctx)
}

func (n *condNode) eval(ctx EvalContext) bool {
	switch n.kind {
	case condCall:
		switch n.name {
		case "always":
			return true
		case "success":
			return ctx.Success
		default:
			return ctx.Failure
		}
	case condNot:
		return !n.l.eval(ctx)
	case condAnd:
		return n.l.eval(ctx) && n.r.eval(ctx)
	case condOr:
		return n.l.eval(ctx) || n.r.eval(ctx)
	case condEq:
		return n.a.value(ctx) == n.b.value(ctx)
	default:
		return n.a.value(ctx) != n.b.value(ctx)
	}
}

func (o condOperand) value(ctx EvalContext) string {
	if !o.env || ctx.Env == nil {
		if o.env {
			return ""
		}
		return o.val
	}
	return ctx.Env(o.val)
}

// ParseCondition parses an if: expression. It executes nothing.
func ParseCondition(text string) (*Condition, error) {
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("the expression is empty")
	}
	if len(text) > maxConditionLen {
		return nil, fmt.Errorf("the expression is longer than %d characters", maxConditionLen)
	}
	toks, err := lexCondition(text)
	if err != nil {
		return nil, err
	}
	c := &Condition{text: text}
	p := &condParser{toks: toks, c: c}
	root, err := p.or()
	if err != nil {
		return nil, err
	}
	if t := p.peek(); t.kind != tokEnd {
		return nil, fmt.Errorf("unexpected %s at position %d", t.describe(), t.pos+1)
	}
	c.root = root
	return c, nil
}

type tokKind int

const (
	tokEnd tokKind = iota
	tokIdent
	tokString
	tokLParen
	tokRParen
	tokNot
	tokAnd
	tokOr
	tokEq
	tokNe
)

type token struct {
	kind tokKind
	text string
	pos  int
}

func (t token) describe() string {
	switch t.kind {
	case tokEnd:
		return "end of expression"
	case tokString:
		return fmt.Sprintf("text %q", t.text)
	default:
		return fmt.Sprintf("%q", t.text)
	}
}

func lexCondition(s string) ([]token, error) {
	var toks []token
	for i := 0; i < len(s); {
		ch := s[i]
		switch {
		case ch == ' ' || ch == '\t':
			i++
		case ch == '(':
			toks, i = append(toks, token{tokLParen, "(", i}), i+1
		case ch == ')':
			toks, i = append(toks, token{tokRParen, ")", i}), i+1
		case strings.HasPrefix(s[i:], "&&"):
			toks, i = append(toks, token{tokAnd, "&&", i}), i+2
		case strings.HasPrefix(s[i:], "||"):
			toks, i = append(toks, token{tokOr, "||", i}), i+2
		case strings.HasPrefix(s[i:], "=="):
			toks, i = append(toks, token{tokEq, "==", i}), i+2
		case strings.HasPrefix(s[i:], "!="):
			toks, i = append(toks, token{tokNe, "!=", i}), i+2
		case ch == '!':
			toks, i = append(toks, token{tokNot, "!", i}), i+1
		case ch == '\'' || ch == '"':
			end := strings.IndexByte(s[i+1:], ch)
			if end < 0 {
				return nil, fmt.Errorf("text starting at position %d is not closed", i+1)
			}
			toks = append(toks, token{tokString, s[i+1 : i+1+end], i})
			i += end + 2
		case isIdentByte(ch):
			start := i
			for i < len(s) && isIdentByte(s[i]) {
				i++
			}
			toks = append(toks, token{tokIdent, s[start:i], start})
		default:
			return nil, fmt.Errorf("unexpected character %q at position %d", ch, i+1)
		}
	}
	return append(toks, token{tokEnd, "", len(s)}), nil
}

func isIdentByte(c byte) bool {
	return c == '_' || c == '.' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

type condParser struct {
	toks  []token
	pos   int
	depth int
	c     *Condition
}

func (p *condParser) peek() token { return p.toks[p.pos] }

func (p *condParser) next() token {
	t := p.toks[p.pos]
	if t.kind != tokEnd {
		p.pos++
	}
	return t
}

func (p *condParser) or() (*condNode, error) {
	left, err := p.and()
	if err != nil {
		return nil, err
	}
	for p.peek().kind == tokOr {
		p.next()
		right, err := p.and()
		if err != nil {
			return nil, err
		}
		left = &condNode{kind: condOr, l: left, r: right}
	}
	return left, nil
}

func (p *condParser) and() (*condNode, error) {
	left, err := p.not()
	if err != nil {
		return nil, err
	}
	for p.peek().kind == tokAnd {
		p.next()
		right, err := p.not()
		if err != nil {
			return nil, err
		}
		left = &condNode{kind: condAnd, l: left, r: right}
	}
	return left, nil
}

func (p *condParser) not() (*condNode, error) {
	p.depth++
	defer func() { p.depth-- }()
	if p.depth > 32 {
		return nil, fmt.Errorf("the expression is nested too deeply")
	}
	t := p.peek()
	switch t.kind {
	case tokNot:
		p.next()
		inner, err := p.not()
		if err != nil {
			return nil, err
		}
		return &condNode{kind: condNot, l: inner}, nil
	case tokLParen:
		p.next()
		inner, err := p.or()
		if err != nil {
			return nil, err
		}
		if c := p.next(); c.kind != tokRParen {
			return nil, fmt.Errorf("expected \")\" at position %d, found %s", c.pos+1, c.describe())
		}
		return inner, nil
	case tokIdent:
		if p.toks[p.pos+1].kind == tokLParen {
			return p.call()
		}
	}
	return p.compare()
}

func (p *condParser) call() (*condNode, error) {
	name := p.next()
	switch name.text {
	case "always", "success", "failure":
	default:
		return nil, fmt.Errorf("unknown function %q at position %d (use always(), success() or failure())", name.text, name.pos+1)
	}
	p.next() // "("
	if c := p.next(); c.kind != tokRParen {
		return nil, fmt.Errorf("%s() takes no arguments (position %d)", name.text, c.pos+1)
	}
	p.c.usesStatus = true
	if name.text != "success" {
		p.c.afterFailure = true
	}
	return &condNode{kind: condCall, name: name.text}, nil
}

func (p *condParser) compare() (*condNode, error) {
	a, err := p.operand()
	if err != nil {
		return nil, err
	}
	op := p.next()
	if op.kind != tokEq && op.kind != tokNe {
		return nil, fmt.Errorf("expected == or != at position %d, found %s", op.pos+1, op.describe())
	}
	b, err := p.operand()
	if err != nil {
		return nil, err
	}
	kind := condEq
	if op.kind == tokNe {
		kind = condNe
	}
	return &condNode{kind: kind, a: a, b: b}, nil
}

func (p *condParser) operand() (condOperand, error) {
	t := p.next()
	switch t.kind {
	case tokString:
		return condOperand{val: t.text}, nil
	case tokIdent:
		name, ok := strings.CutPrefix(t.text, "env.")
		if !ok || !envName.MatchString(name) {
			return condOperand{}, fmt.Errorf("%q at position %d is not a value: write text in quotes or env.NAME", t.text, t.pos+1)
		}
		return condOperand{env: true, val: name}, nil
	}
	return condOperand{}, fmt.Errorf("expected a value at position %d, found %s", t.pos+1, t.describe())
}
