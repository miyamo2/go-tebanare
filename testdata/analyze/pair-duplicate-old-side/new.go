package user

func (u *User) Name() string { return u.name }

func load() error { return nil }

func load() error {
	err := open()
	if err != nil {
		return err
	}
	return nil
}
