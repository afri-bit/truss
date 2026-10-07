// Package engine drives a validated pipeline through its step lifecycle.
//
// The engine runs steps sequentially, delegates each step to an Executor,
// and reports progress exclusively as typed events delivered to an
// EventSink. It never formats output for humans; renderers and sinks do.
//
// Dependencies point inward: this package MUST NOT import the daemon, the
// HTTP API, store implementations, or anything under internal/. These
// boundaries are enforced by depguard (see .golangci.yml).
package engine
