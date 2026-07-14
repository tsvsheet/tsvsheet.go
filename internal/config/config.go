// Package config provides an in-memory configuration store with get, set, and
// list operations.
//
// The Store wraps a map, so its value-receiver methods still observe and mutate
// shared entries. The package holds no CLI or orchestration logic and is reusable
// by any domain that needs configuration access. A real application would back
// this with files, a database, or a remote service behind the same operations.
package config

import "strings"

type (
	// Key identifies a configuration entry.
	Key string
	// Value is a configuration entry's value.
	Value string
	// Prefix filters keys by a leading substring.
	Prefix string
)

// Store is an in-memory configuration store.
type Store struct {
	data map[Key]Value
}

// NewStore returns a store seeded with example data.
func NewStore() Store {
	return Store{data: map[Key]Value{
		"app.name":        "template.cli",
		"app.version":     "1.0.0",
		"database.host":   "localhost",
		"database.port":   "5432",
		"log.level":       "info",
		"feature.enabled": "true",
	}}
}

// Get returns the value stored under key and whether it exists.
func (s Store) Get(key Key) (Value, bool) {
	value, ok := s.data[key]
	return value, ok
}

// Set stores value under key, returning the previous value and whether one
// existed. It mutates the shared map through the value receiver.
func (s Store) Set(key Key, value Value) (Value, bool) {
	previous, existed := s.data[key]
	s.data[key] = value
	return previous, existed
}

// List returns the entries whose key matches prefix; an empty prefix returns all.
func (s Store) List(prefix Prefix) map[Key]Value {
	matches := make(map[Key]Value)
	for key, value := range s.data {
		if matchesPrefix(key, prefix) {
			matches[key] = value
		}
	}
	return matches
}

// matchesPrefix reports whether key starts with prefix (empty prefix matches all).
func matchesPrefix(key Key, prefix Prefix) bool {
	return prefix == "" || strings.HasPrefix(string(key), string(prefix))
}
