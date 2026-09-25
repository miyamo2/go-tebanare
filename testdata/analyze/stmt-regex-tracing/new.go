package handler

import "context"

func handle(ctx context.Context, id string) error {
	ctx, span := tracer.Start(ctx, "handle")
	defer span.End()

	return process(ctx, id)
}
