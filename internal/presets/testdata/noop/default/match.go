package p

func (*BadExpr) exprNode() {}

func (s Set[T]) sealed() {}

func (t *noopTracer) Flush() {}

func (s *Server) Shutdown() { /* TODO: implement */ }

func (_ nopLogger) Sync() {}

func (c *Cache[K, V]) reset() {}

// Stop does nothing.
func (t *noopTimer) Stop() {
}

func (t *noopTimer) Reset() {
	// The timer never fires.
}

func (t *noopTimer) Close() {} // trailing comment
