package user

// Name returns the name.
// See: https://example.com/names
func (u *User) Name() string { return u.name }

//go:noinline
func (u *User) Email() string { return u.email }

// Phone returns the phone number.
//
//nolint:errcheck
func (u *User) Phone() string { return u.phone }

//export UserAge
func (u *User) Age() int { return u.age }

//extern user_id
func (u *User) ID() int64 { return u.id }
