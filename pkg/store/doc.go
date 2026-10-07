// Package store persists pipeline run history.
//
// It defines the storage contract used by the daemon and by direct mode.
// Implementations (an SQLite store built on the pure-Go modernc.org/sqlite
// driver, and an in-memory store for direct mode) live in sub-packages so
// that consumers depend only on the contract.
//
// The engine MUST NOT import this package or its implementations; run
// history is recorded by an event sink that adapts engine events.
package store
