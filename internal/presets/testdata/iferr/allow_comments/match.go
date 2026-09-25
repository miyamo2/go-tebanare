package p

func f() error {
	if err != nil { // the caller logs it
		return err
	}
	if err != nil {
		// the caller logs it
		return err
	}
	if err != nil {
		return err
	} // the caller logs it
	if err != nil {
		return err
	}
}
