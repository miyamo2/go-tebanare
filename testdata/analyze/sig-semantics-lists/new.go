package util

func Map[T, U any](xs []T, f func(T) U) []U { return mapSlice(xs, f) }

func (c *Conn) Close() { c.close() }

func (f *File) Close() error { return f.close() }

func WrapMsg(msg string, err error) { wrap(msg, err) }

func WrapErr(err error) { wrap("", err) }
