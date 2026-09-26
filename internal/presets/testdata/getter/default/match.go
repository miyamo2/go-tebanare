package p

func (u *User) Name() string { return u.name }

func (u User) ID() (id int64) { return (u.id) }

func (s *Stack[T]) Len() int { return s.n }

// Timeout returns the request timeout.
func (u *User) Timeout() time.Duration { return u.cfg.timeout }

func (c *Cache[K, V]) Size() int { return c.size }

func (u *User) Deep() int { return u.a.b.c.d }

func (u *User) Parens() int { return ((u).cfg).limit }

func (u *User) Multiline() string {
	return u.name
}

func (u *User) Closer() func() error { return u.Close }

func (o *Other) Close() error { return o.err }
