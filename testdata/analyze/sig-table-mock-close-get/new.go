package store

import "context"

func (m *MockUserRepo) Get(ctx context.Context, id string) (*User, error) { return m.get(ctx, id) }

func (mr *MockUserRepoMockRecorder) Get(ctx, id any) *Call { return mr.record("Get", ctx, id) }

func (m MockOrderRepo) Get(ctx context.Context, id string) (*Order, error) { return m.get(ctx, id) }

func (c *Conn) Close() { c.close() }

func (f *File) Close() error { return f.close() }

func (c Cache) Get(k T) T { return c.m[k] }

func (c *Pool[T]) Get(k T) T { return c.m[k] }
