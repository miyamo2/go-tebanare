package scope

type buf [len("abc")]byte

var sizes = []int{
	len("abc"),
}

const (
	size = len("abc")
)

var hook = func(b [len("abc")]byte) {
	debug()
}

func fill(b [len("abc")]byte) {
	debug()
	_ = []int{
		len("abc"),
	}
}
