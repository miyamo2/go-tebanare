package p

func f() error {
	if fooErr != nil {
		return nil, barErr
	}
	if parseErr != nil {
		return nil, err
	}
	if parseError != nil {
		return parseError
	}
	if e != nil {
		return e
	}
	if e12 != nil {
		return e12
	}
	if r.parseErr != nil {
		return r.parseErr
	}
	if error != nil {
		return error
	}
}
