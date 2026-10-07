// Package executor runs a single step on a concrete backend.
//
// v0.1.0 provides two backends: a host executor based on os/exec and a
// container executor based on the Docker Engine API. Executors preserve
// exit codes, honor context cancellation, and make no shell-specific or
// platform-specific assumptions beyond those declared by the step.
//
// This package MUST NOT import engine, store, api, or internal packages.
package executor
