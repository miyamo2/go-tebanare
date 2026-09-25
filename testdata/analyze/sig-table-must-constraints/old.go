package store

func Must[V any](v V, err error) V { return must(v, err) }
