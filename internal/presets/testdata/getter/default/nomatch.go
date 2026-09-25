package p

func (u *User) Title() string { return strings.TrimSpace(u.title) }

func (u *User) NameOr(def string) string { return u.name }

func (u *User) Pair() (string, int) { return u.name, u.age }

func (u *User) First() string { return u.items[0] }

func (u *User) Limit() int { return u.cfg.Limit() }

func (*User) Kind() string { return kind }

func (u *User) Age() int { return u.age /* TODO */ }

func (_ *User) Blank() int { return _.n }

func (u *User) Self() *User { return u }

func (u *User) Other() int { return v.n }

func (u *User) Pointer() *int { return &u.n }

func (u *User) Deref() int { return *u.p }

func (u *User) Converted() int64 { return int64(u.n) }

func (u *User) Variadic(xs ...int) int { return u.n }

func (u *User) NoResult() { return }

func (u *User) Bodyless() int

func (u *User) TwoStatements() int {
	u.reads++
	return u.n
}

func (u *User) Trailing() int { return u.n } // trailing comment

func (u *User) Inner() int {
	// inner comment
	return u.n
}

func (u *User) Close() error { u.closed = true; return nil }

func (u *User) Closer() func() error { return u.Close }

func Name(u *User) string { return u.name }
