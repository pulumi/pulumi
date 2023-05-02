package pulumix

import (
	"context"
	"fmt"
	"strconv"
)

type Applicator[U, T any] interface {
	~func(T) U |
		~func(T) (U, error) |
		~func(context.Context, T) U |
		~func(context.Context, T) (U, error)
}

func Apply[U any, F Applicator[U, T], T any](t T, f F) (U, error) {
	return ApplyContext[U](context.Background(), t, f)
}

func ApplyContext[U any, F Applicator[U, T], T any](ctx context.Context, t T, f F) (U, error) {
	switch f := any(f).(type) {
	case func(T) U:
		return f(t), nil
	case func(T) (U, error):
		return f(t)
	case func(context.Context, T) U:
		return f(ctx, t), nil
	case func(context.Context, T) (U, error):
		return f(ctx, t)
	default:
		panic("invalid function type")
	}
}

func UseApply() {
	if x, err := Apply[string](42, strconv.Itoa); err != nil {
		panic(err)
	} else {
		fmt.Println(x)
	}

	if x, err := Apply[int]("42", strconv.Atoi); err != nil {
		panic(err)
	} else {
		fmt.Println(x)
	}

	x, err := Apply[string](42, func(context.Context, int) (string, error) {
		return "", nil
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(x)
}
