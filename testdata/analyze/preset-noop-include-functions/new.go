package trace

func noop() {}

func setup() { register() }

func (t *noopTracer) Flush() {}
