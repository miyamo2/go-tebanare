package config

func run() (time.Time, error) {
	err := open()
	if err != nil { // the caller logs it
		return time.Time{}, err
	}
	if err := load(); err != nil {
		return time.Time{}, err
	}
	err = read()
	if err != nil {
		return
	}
	err = parse()
	if err != nil {
		return time.Now(), err
	}
	if err != nil {
		return func() {}, err
	}
	if err != nil {
		return <-ch, err
	}
	return time.Now(), nil
}
