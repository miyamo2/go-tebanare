package user

type User struct {
	name  string
	email string
}

func (u *User) Name() string {
	return u.name
}

func (u *User) Rename(name string) {
	u.name = name
}
