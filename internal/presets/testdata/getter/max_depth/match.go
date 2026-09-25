package p

func (u *User) Name() string { return u.name }

func (u User) ID() (id int64) { return (u.id) }
