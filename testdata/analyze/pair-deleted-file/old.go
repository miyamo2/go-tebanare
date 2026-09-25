package user

// Name returns the name.
func (u *User) Name() string { return u.name }

func (u *User) Save() error {
	err := u.store.Put(u)
	if err != nil {
		return err
	}
	return nil
}
