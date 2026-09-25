package user

type User struct {
	name  string
	email string
}

func (u *User) Rename(name string) {
	u.name = name
}

func (u *User) Email() string {
	return u.email
}
