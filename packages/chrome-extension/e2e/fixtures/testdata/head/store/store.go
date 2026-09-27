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
	owner string
	items map[string]int
}

// Owner returns the store owner.
func (s *Store) Owner() string {
	return s.owner
}

// Put stores v under k.
func (s *Store) Put(k string, v int) error {
	err := validate(k)
	if err != nil {
		return err
	}
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

// validate returns ErrEmptyKey for an empty key.
func validate(k string) error {
	if k == "" {
		return ErrEmptyKey
	}
	return nil
}
