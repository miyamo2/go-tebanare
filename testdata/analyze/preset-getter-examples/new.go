package user

import (
	"strings"
	"time"
)

func (u *User) Name() string  { return u.name }
func (u User) ID() (id int64) { return (u.id) }
func (s *Stack[T]) Len() int  { return s.n }

// Timeout returns the request timeout.
func (u *User) Timeout() time.Duration { return u.cfg.timeout }

func (u *User) Title() string            { return strings.TrimSpace(u.title) }
func (u *User) NameOr(def string) string { return u.name }
func (u *User) Pair() (string, int)      { return u.name, u.age }
func (u *User) First() string            { return u.items[0] }
func (u *User) Limit() int               { return u.cfg.Limit() }
func (*User) Kind() string               { return kind }
func (u *User) Age() int                 { return u.age /* TODO */ }
func (u *User) NameFunc() func() string  { return u.Name }
