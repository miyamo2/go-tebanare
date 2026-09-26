package user

func (u *User) Name() string { return u.name }

func (u *User) Email() string { return u.email }
func (u *User) Phone() string { return u.phone }

/* Getters for the profile. */

func (u *User) Age() int { return u.age }

func (u *User) Save() error {
	return u.store.Put(u)
}

func (u *User) ID() string { return u.id }
