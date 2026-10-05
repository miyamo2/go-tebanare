package store

import "context"

func (r *Repository[E]) Save(ctx context.Context, v E) error { return r.put(ctx, v) }

func Map[A, B any](xs []A, f func(A) B) []B { return mapSlice(xs, f) }

func Must[V any](v V, err error) V { return must(v, err) }

func Pair[A, B comparable](a A, b B) { pair(a, b) }

func New() *Store { return &Store{} }

func NewRepository[T any](db DB) *Repository[T] { return &Repository[T]{db: db} }

func (f *Factory) NewStore() *Store { return f.store() }
