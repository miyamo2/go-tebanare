package p

func f() error {
	if err := load(); err != nil {
		return err
	} // the caller logs it
	if err := load(); err != nil {
		return fmt.Errorf("load: %w", err)
	}
	if err := load(); err != nil {
		return
	}
	if err := load(); err != nil {
		return err
	} else {
		return nil
	}
}
