package user

import "time"

func (u *User) Name() string { return u.name }

func (u *User) Timeout() time.Duration { return u.cfg.timeout }
