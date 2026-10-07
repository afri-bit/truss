// Package pipeline parses and validates TRUSS pipeline definitions.
//
// It owns the versioned pipeline schema (public contract) and turns YAML
// into a validated, executor-independent model. Parsing never executes
// anything. Validation errors carry source positions (line and column).
//
// Allowed imports: the standard library and YAML parsing only. This
// package MUST NOT import engine, executor, store, api, or internal
// packages.
package pipeline
