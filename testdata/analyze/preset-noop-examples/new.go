package ast

func (*BadExpr) exprNode()   {}
func (s Set[T]) sealed()     {}
func (t *noopTracer) Flush() {}
func (s *Server) Shutdown()  { /* TODO: implement */ }

func (t *Tracer) Flush()                            { t.buf.Reset() }
func (nopLogger) Printf(format string, args ...any) {}
func (nopCloser) Close() error                      { return nil }
func noop()                                         {}
func (t *Timer) stop()
