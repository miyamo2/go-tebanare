package store

func Must[V comparable](v V, err error) V { return must(v, err) }
