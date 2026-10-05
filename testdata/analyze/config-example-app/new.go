package store

import (
	"context"
	"fmt"
	"log/slog"
)

// Name returns the user name.
func (u *User) Name() string { return u.name }

func (u *User) Validate() {}

func (u User) String() string { return fmt.Sprintf("User(%s)", u.id) }

func (e Entry) String() (string, error) { return e.id, nil }

func (m *MockClock) Now() int64 { return m.ctrl.Call(m, "Now")[0].(int64) }

func (r *Repository[T]) FindByID(ctx context.Context, id string) (T, error) { return r.get(ctx, id) }

func (r Repository[T]) FindAll(ctx context.Context) ([]T, error) { return r.all(ctx) }

func (r *Repository[T]) Save(ctx context.Context, v T) error {
	ctx, span := tracer.Start(ctx, "Save")
	defer span.End()
	slog.DebugContext(ctx, "save", "value", v)
	slog.Debug("save", "token", r.token)
	slog.Info("save", "value", v)
	b, encodeErr := encode(v)
	if encodeErr != nil {
		return encodeErr
	}
	return r.db.Put(ctx, b)
}

func Must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
