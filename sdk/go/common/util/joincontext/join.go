package joincontext

import (
	"context"
	"reflect"
	"time"
)

// joinContext implements a context that completes when any one of a list of contexts completes.
type joinContext struct {
	contexts []context.Context

	done <-chan struct{}
	err  error
}

// wait2 is a specialized implementation of wait that handles exactly two contexts.
func wait2(a, b context.Context) error {
	select {
	case <-a.Done():
		return a.Err()
	case <-b.Done():
		return b.Err()
	}
}

// wait returns Context.Err() for the first of the input contexts that completes.
func wait(contexts []context.Context) error {
	if len(contexts) == 2 {
		return wait2(contexts[0], contexts[1])
	}

	cases := make([]reflect.SelectCase, len(contexts))
	for i, ctx := range contexts {
		cases[i] = reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(ctx.Done()),
		}
	}
	chosen, _, _ := reflect.Select(cases)
	return contexts[chosen].Err()
}

// Deadline returns the earliest deadline (if any) in the joinContext's list of contexts.
func (ctx *joinContext) Deadline() (deadline time.Time, ok bool) {
	soonest, ok := time.Time{}, false
	for _, ctx := range ctx.contexts {
		deadline, hasDeadline := ctx.Deadline()
		if hasDeadline && !ok || deadline.Before(soonest) {
			soonest, ok = deadline, true
		}
	}
	return soonest, ok
}

func (ctx *joinContext) Done() <-chan struct{} {
	return ctx.done
}

func (ctx *joinContext) Err() error {
	return ctx.err
}

// Value returns the first non-nil value associated with this context for key, or nil if no value is associated with
// key. Successive calls to Value with the same key returns the same result.
func (ctx *joinContext) Value(key interface{}) interface{} {
	for _, c := range ctx.contexts {
		if v := c.Value(key); v != nil {
			return v
		}
	}
	return nil
}

// Join returns a context that completes when at least one of the input contexts has completed. If any of these contexts
// has a deadline, the deadline of the result will be the soonest deadline. The implementation of Value iterates the
// slice of context and returns the first non-nil value for the given key, if any.
func Join(first context.Context, rest ...context.Context) context.Context {
	if len(rest) == 0 {
		return first
	}

	contexts := append([]context.Context{first}, rest...)
	done := make(chan struct{})
	ctx := &joinContext{
		contexts: contexts,
		done:     done,
	}
	go func() {
		ctx.err = wait(contexts)
		close(done)
	}()
	return ctx
}
