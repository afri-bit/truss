// Package api defines the versioned HTTP API (/v1) between the truss CLI
// and the trussd daemon.
//
// It holds request, response, and server-sent event payload types plus a
// client. The API is a public contract: changes require an approved spec
// and an ADR. Payloads MUST NOT expose Go-specific encodings so that
// non-Go clients can consume them.
//
// This package MUST NOT import the engine's internals or anything under
// internal/.
package api
