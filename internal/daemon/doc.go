// Package daemon implements trussd: the binding registry, the cron and
// manual trigger scheduler (one lock per binding), and the HTTP + SSE
// server that exposes the /v1 API.
//
// The daemon composes the engine, executors, store, and sinks; nothing in
// pkg/ may import it.
package daemon
