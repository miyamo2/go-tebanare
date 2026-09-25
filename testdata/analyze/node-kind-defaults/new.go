package flags

func run(x int) {
	{
		x++
	}
	flags := []bool{
		debugMode,
	}
	names := []string{
		"debug",
	}
	use(flags, names)
}
