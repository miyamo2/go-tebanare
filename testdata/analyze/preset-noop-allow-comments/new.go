package ast

func (*BadExpr) exprNode() {}

func (s *Server) Shutdown() { /* TODO: implement */ }

func (*BadStmt) stmtNode() {} // marker method

// flush does nothing: noopTracer keeps no buffer.
func (t *noopTracer) flush() {}
