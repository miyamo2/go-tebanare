package handler

import "context"

func handle(ctx context.Context, id string) error {
	return process(ctx, id)
}
