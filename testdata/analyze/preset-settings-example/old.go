package user

func noop() {}

func (u *User) Name() string { return u.name }

func (u *User) Load() error {
	parseErr := u.parse()
	if parseErr != nil {
		return parseErr
	}
	return nil
}
