package store

import "context"

func (r *Repository[E]) Save(ctx context.Context, v *E) error { return r.put(ctx, v) }

func Map[A any, B comparable](xs []A, f func(A) B) []B { return mapSlice(xs, f) }

func Must[V any](v V, err error) *V { return must(v, err) }

func Pair[A any, B comparable](a A, b B) { pair(a, b) }
