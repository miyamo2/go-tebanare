package user

type User struct {
	name string
}

func (u *User) Name() string {
	return u.name
}
