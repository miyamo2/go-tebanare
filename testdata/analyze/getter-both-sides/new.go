package user

type User struct {
	fullName string
}

// Name returns the full name of the user.
func (u *User) Name() string {
	return u.fullName
}
