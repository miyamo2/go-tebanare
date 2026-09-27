// Package store is a synthetic fixture for the extension's end-to-end tests.
package store

import (
	"errors"
	"sync"
)

// ErrEmptyKey reports an empty key.
var ErrEmptyKey = errors.New("empty key")

// Store keeps values by key.
type Store struct {
	mu    sync.Mutex
	items map[string]int
}

// Put stores v under k.
func (s *Store) Put(k string, v int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[k] = v
	return nil
}

// Len returns the number of items.
func (s *Store) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.items)
}
