package user

type User struct {
	name string
}

func (u *User) Name() string {
	if u.name == "" {
		return "anonymous"
	}
	return u.name
}
