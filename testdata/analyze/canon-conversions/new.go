package repo

func (r *Repository[T]) Find(ctx context.Context, id string) (T, error) {
	log.Printf(
		"find %s",
		id,
	)
	ctx = withID(ctx, id)
	if err := r.check(
		ctx,
	); err != nil {
		return *new(T), fmt.Errorf("check: %w",
			err)
	}
	ctx = withRetry(ctx)
	v := Config{
		A: 1,
		B: []int{
			1, 2,
		},
	}
	return r.find(ctx, v)
}
