// Copyright 2016 Marapongo, Inc. All rights reserved.

package retry

import (
	"context"
	"time"
)

type Acceptor struct {
	Accept Acceptance
	Delay  time.Duration
}

type Acceptance func() (bool, error)

// Until waits until the acceptor accepts the current condition, or the context expires, whichever comes first.  A
// return boolean of true means the acceptor eventually accepted; a non-nil error means the acceptor returned an error.
// If an acceptor accepts a condition after the context has expired, we ignore the expiration and return the condition.
func Until(ctx context.Context, acceptor Acceptor) (bool, error) {
	expired := false

	// If the context expires before the waiter has accepted, return.
	go func() {
		<-ctx.Done()
		expired = true
	}()

	// Loop until the condition is accepted, or the context expires, whichever comes first.
	for !expired {
		if b, err := acceptor.Accept(); b || err != nil {
			return b, err
		}
		time.Sleep(acceptor.Delay)
	}

	return false, nil
}

// UntilDeadline creates a child context with the given deadline, and then invokes the above Until function.
func UntilDeadline(ctx context.Context, acceptor Acceptor, deadline time.Time) (bool, error) {
	ctx, _ = context.WithDeadline(ctx, deadline)
	return Until(ctx, acceptor)
}

// UntilTimeout creates a child context with the given timeout, and then invokes the above Until function.
func UntilTimeout(ctx context.Context, acceptor Acceptor, timeout time.Duration) (bool, error) {
	ctx, _ = context.WithTimeout(ctx, timeout)
	return Until(ctx, acceptor)
}
