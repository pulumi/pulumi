// Copyright 2016 Pulumi, Inc. All rights reserved.

package retry

import (
	"context"
	"time"
)

type Acceptor struct {
	Accept   Acceptance     // a function that determines when to proceed.
	Progress func(int) bool // an optional progress function.
	Delay    *time.Duration // an optional delay duration.
	Backoff  *float64       // an optional backoff multiplier.
}

const (
	DefaultDelay   time.Duration = 250 * time.Millisecond // by default, delay by 250ms
	DefaultBackoff float64       = 2.0                    // by default, backoff by 2.0
)

type Acceptance func() (bool, error)

// Until waits until the acceptor accepts the current condition, or the context expires, whichever comes first.  A
// return boolean of true means the acceptor eventually accepted; a non-nil error means the acceptor returned an error.
// If an acceptor accepts a condition after the context has expired, we ignore the expiration and return the condition.
func Until(ctx context.Context, acceptor Acceptor) (bool, error) {
	expired := false

	// Prepare our delay and backoff variables.
	var delay time.Duration
	if acceptor.Delay == nil {
		delay = DefaultDelay
	} else {
		delay = *acceptor.Delay
	}
	var backoff float64
	if acceptor.Backoff == nil {
		backoff = DefaultBackoff
	} else {
		backoff = *acceptor.Backoff
	}

	// If the context expires before the waiter has accepted, return.
	go func() {
		<-ctx.Done()
		expired = true
	}()

	// Loop until the condition is accepted, or the context expires, whichever comes first.
	tries := 1
	for !expired {
		if b, err := acceptor.Accept(); b || err != nil {
			return b, err
		}
		if acceptor.Progress != nil && !acceptor.Progress(tries) {
			break // progress function asked to quit.
		}
		time.Sleep(delay)
		delay = time.Duration(float64(delay) * backoff)
		tries++
	}

	return false, nil
}

// UntilDeadline creates a child context with the given deadline, and then invokes the above Until function.
func UntilDeadline(ctx context.Context, acceptor Acceptor, deadline time.Time) (bool, error) {
	var cancel context.CancelFunc
	ctx, cancel = context.WithDeadline(ctx, deadline)
	b, err := Until(ctx, acceptor)
	cancel()
	return b, err
}

// UntilTimeout creates a child context with the given timeout, and then invokes the above Until function.
func UntilTimeout(ctx context.Context, acceptor Acceptor, timeout time.Duration) (bool, error) {
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, timeout)
	b, err := Until(ctx, acceptor)
	cancel()
	return b, err
}
