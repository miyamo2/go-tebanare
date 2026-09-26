package user

// Close releases nothing.
func (c *Conn) Close() {}

// Name returns the name.
func (u *User) Name() string { return u.name }
