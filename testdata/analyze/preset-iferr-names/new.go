package config

func parse(r *Reader) (*Config, error) {
	c, parseErr := decode(r)
	if parseErr != nil {
		return nil, parseErr
	}
	fooErr, barErr := check(c)
	if fooErr != nil {
		return nil, barErr
	}
	if r.err != nil {
		return nil, r.err
	}
	err := validate(c)
	if err != nil {
		return nil, err
	}
	return c, nil
}
