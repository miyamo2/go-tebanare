package user

// Close releases the connection.
func (c *Conn) Close() error { return c.conn.Close() }

// Name returns the name.
func (u *User) Name() string { return u.name }

// String implements fmt.Stringer.
func (u *User) String() string { return fmt.Sprint(u.id) }
