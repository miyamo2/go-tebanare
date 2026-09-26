package p

func (t *Tracer) Flush() { t.buf.Reset() }

func (nopLogger) Printf(format string, args ...any) {}

func (nopCloser) Close() error { return nil }

func noop() {}

func (t *Timer) stop()

func (nopCloser) Wait() (err error) { return }

func (s *Server) Nothing() {
	{
	}
}

func init() {}
