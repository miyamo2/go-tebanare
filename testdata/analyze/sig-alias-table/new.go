package handler

import (
	"context"
	. "context"
	ctx2 "context"
)

type Ctx = context.Context

func HandleA(ctx context.Context) error { return nil }

func HandleB(ctx ctx2.Context) error { return nil }

func HandleC(ctx Context) error { return nil }

func HandleD(ctx Ctx) error { return nil }
