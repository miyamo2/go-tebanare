package user

func (m *Mutex) Lock() { m.lock() }

func (s spinLock) Lock() { s.lock() }

func (u *User) GetX() string { return u.x }

func (u *User) Get() string { return u.x }

func (u *User) GetXY() string { return u.xy }

func (u *User) Getter() string { return u.getter }

func (u *User) SetNameOrDefault(name string) { u.name = name }

func (u *User) ResetName() { u.name = "" }
