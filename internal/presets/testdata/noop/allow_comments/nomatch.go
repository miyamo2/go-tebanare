package p

func (s *Server) Shutdown() { /* TODO: implement */ }

func (t *noopTimer) Reset() {
	// The timer never fires.
}

func (t *noopTimer) Close() {} // trailing comment

func noop() {}
