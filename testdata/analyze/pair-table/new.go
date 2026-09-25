package shape

func (p Point) String() string {
	return fmt.Sprintf("(%d, %d)", p.X, p.Y)
}

func (s Square) String() (string, error) { return "square", nil }

func (t Triangle) String() string { return "triangle" }

func (l Line) String() string { return "line" }
