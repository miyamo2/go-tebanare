package handler

func handle(ctx context.Context) {
	// Start a span for the request.
	// It ends when handle returns.
	ctx, span := tracer.Start(ctx, "handle")
	defer span.End()
	process(ctx)

	// Dump the request.

	log.Debug("request", ctx)
	process(ctx) // process again
	log.Debug("processed")
	process(ctx)
	/* Dump the result. */
	log.Debug("result")
}
