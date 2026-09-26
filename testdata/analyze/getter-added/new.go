package user

type User struct {
	name string
}

// Name returns the name of the user.
func (u *User) Name() string {
	return u.name
}

func (u *User) SetName(name string) {
	u.name = name
}
