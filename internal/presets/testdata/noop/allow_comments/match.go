package p

func (*BadExpr) exprNode() {}

func (t *noopTracer) Flush() {}

// Stop does nothing.
func (t *noopTimer) Stop() {
}
