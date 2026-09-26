package p

func f() error {
	if err := load(); err != nil {
		return err
	}
	if err := load(); err != nil { // the header line stays visible
		return err
	}
	if err := load(); err != nil {
		// a comment above return stays visible
		return err
	}
	if v, err := load(); err != nil {
		return nil, err
	}
	if err != nil {
		return err
	}
}
