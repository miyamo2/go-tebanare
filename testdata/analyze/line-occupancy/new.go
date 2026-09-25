package calc

import "log"

func run() {
	f := func(x int) int { return x + 1 } // the return statement shares its line
	x := compute(log.Debug("..."))        // the call shares its line
	log.Debug("...")                      // the call fills its line
	tests := []tc{
		{name: "legacy", in: 1, want: 2}, // only the element line is hidden
	}
}
