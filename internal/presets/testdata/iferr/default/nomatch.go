package p

func f() (int, error) {
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
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
	if fooErr != nil {
		return nil, barErr
	}
	if nil != err {
		return err
	}
	if err != nil {
		return err
	} // the caller logs it
	if err != nil {
		return err
	} else {
		return nil
	}
	if err != nil {
		panic(err)
	}
	if err != nil {
		return errors.Wrap(err, "load")
	}
	if err != nil {
		return compute(), err
	}
	if err != nil {
		return func() {}, err
	}
	if err != nil {
		return <-ch, err
	}
}
