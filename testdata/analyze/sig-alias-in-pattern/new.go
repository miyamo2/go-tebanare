package handler

import (
	"context"
	ctx2 "context"
)

func HandleA(ctx context.Context) error { return nil }

func HandleB(ctx ctx2.Context) error { return nil }
