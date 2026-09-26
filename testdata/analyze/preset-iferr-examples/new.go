package config

func load() (*Config, error) {
	err := open()
	if err != nil {
		return err
	}
	err = read()
	if err != nil {
		return nil, err
	}
	err = decode()
	if err != nil {
		return *new(T), err
	}
	err = parse()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	err = check()
	if err != nil {
		log.Printf("load: %v", err)
		return err
	}
	if err := load(); err != nil {
		return err
	}
	if err != nil {
		return
	}
	if err != nil { // the caller logs it
		return err
	}
	if err != nil {
		return err
	} // the caller logs it
	if nil != err {
		return err
	}
	if err != nil {
		return err
	} else {
		cleanup()
	}
	err = stamp()
	if err != nil {
		return time.Now(), err
	}
	if err != nil {
		return func() {}, err
	}
	if err != nil {
		return <-ch, err
	}
	return c, nil
}
