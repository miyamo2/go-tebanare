package shape

func (p Point) String() string { return fmt.Sprintf("(%d, %d)", p.x, p.y) }

func (s Square) String() string { return "square" }

func (c Circle) String() string { return "circle" }

func (l Line) String() (string, error) { return "line", nil }
