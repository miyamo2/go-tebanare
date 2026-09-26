package p

func (u *User) Timeout() time.Duration { return u.cfg.timeout }

func (u *User) Parens() int { return ((u).cfg).limit }
