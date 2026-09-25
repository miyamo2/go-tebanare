package calc

func run() {
	values := []int{
		debug(1), debug(2),
		debug(3), // kept for the report
	}
	/* first */ debug(4)
	debug(
		5,
	).Wait()
	use(values)
}
