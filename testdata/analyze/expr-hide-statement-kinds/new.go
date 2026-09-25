package server

func handle(ch chan error) (error, error) {
	logger.Debug("expr")
	_ = logger.Debug("assign")
	err := logger.Debug("define")
	var dbg = logger.Debug("var")
	var typed error = logger.Debug("var with type")
	defer logger.Debug("defer")
	go logger.Debug("go")
	ch <- logger.Debug("send")
	x.y = logger.Debug("field")
	var s fmt.Stringer = logger.Debug("qualified type")
	fmt.Println(logger.Debug("argument"))
	if err := logger.Debug("if"); err != nil {
		return err, nil
	}
	return logger.Debug("return"), nil
}
