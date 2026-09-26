package p

func f() error {
	if err != nil { // the caller logs it
		return fmt.Errorf("load config: %w", err)
	}
	if err := load(); err != nil { // the caller logs it
		return err
	}
}
