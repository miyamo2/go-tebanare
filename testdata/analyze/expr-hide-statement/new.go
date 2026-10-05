package server

func handle(state State) {
	_ = logger.Debug("start")
	if logger.DebugEnabled() {
		dump(state)
	}
}
