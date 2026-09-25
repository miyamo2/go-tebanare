package store

import "context"

func (u User) String() string { return u.name }

func String() string { return "store" }

func (p *Point) String() string { return p.label }

func (a Account) String() (string, error) { return a.id, nil }

func (r *Repository[T]) FindAll(ctx context.Context) ([]T, error) { return r.all(ctx) }

func (r *Repository[T]) FindByID(ctx context.Context, id string) (T, error) { return r.get(ctx, id) }

func (r Repository[T]) FindByName(ctx context.Context, name string) (T, error) {
	return r.get(ctx, name)
}

func (c *Cache[K, V]) Put(k K, v V) { c.m[k] = v }

func (c *Cache[T]) Get(k string) T { return c.m[k] }

func (c *Cache) Len() int { return len(c.m) }
