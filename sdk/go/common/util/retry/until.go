// Copyright 2016-2018, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package retry

import (
	"context"
	"time"
)

type Acceptor struct {
	Accept   Acceptance     // a function that determines when to proceed.
	Delay    *time.Duration // an optional delay duration.
	Backoff  *float64       // an optional backoff multiplier.
	MaxDelay *time.Duration // an optional maximum delay duration.
}

// Acceptance is meant to accept a condition.
// It returns true when this condition has succeeded, and false otherwise
// (to which we respond by waiting and retrying after a certain period of time).
// If a non-nil error is returned, retrying halts.
// The interface{} data may be used to return final values to the caller.
//
// Try specifies the attempt number,
// zero indicating that this is the first attempt with no retries.
type Acceptance func(try int, nextRetryTime time.Duration) (success bool, result interface{}, err error)

const (
	DefaultDelay    time.Duration = 100 * time.Millisecond // by default, delay by 100ms
	DefaultBackoff  float64       = 1.5                    // by default, backoff by 1.5x
	DefaultMaxDelay time.Duration = 5 * time.Second        // by default, no more than 5 seconds
)

// Retryer provides the ability to run and retry a fallible operation
// with exponential backoff.
type Retryer struct {
	// Returns a channel that will send the time after the duration elapses.
	//
	// Defaults to time.After.
	After func(time.Duration) <-chan time.Time
}

// Until runs the provided acceptor until one of the following conditions is met:
//
//   - the operation succeeds: returns true and the result
//   - the context expires: returns false and no result or errors
//   - the operation returns an error: returns an error
//
// Note that the number of attempts is not limited.
// The Acceptance function is responsible for determining
// when to stop retrying.
func (r *Retryer) Until(ctx context.Context, acceptor Acceptor) (bool, interface{}, error) {
	timeAfter := time.After
	if r.After != nil {
		timeAfter = r.After
	}

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
	var maxDelay time.Duration
	if acceptor.MaxDelay == nil {
		maxDelay = DefaultMaxDelay
	} else {
		maxDelay = *acceptor.MaxDelay
	}

	// Loop until the condition is accepted or the context expires, whichever comes first.
	try := 0
	for {
		if delay > maxDelay {
			delay = maxDelay
		}

		// Try the acceptance condition; if it returns true, or an error, we are done.
		b, data, err := acceptor.Accept(try, delay)
		if b || err != nil {
			return b, data, err
		}

		// Wait for delay or timeout.
		select {
		case <-timeAfter(delay):
			// Continue on.
		case <-ctx.Done():
			return false, nil, nil
		}

		delay = time.Duration(float64(delay) * backoff)
		try++
	}
}

// Until waits until the acceptor accepts the current condition, or the context expires, whichever comes first.  A
// return boolean of true means the acceptor eventually accepted; a non-nil error means the acceptor returned an error.
// If an acceptor accepts a condition after the context has expired, we ignore the expiration and return the condition.
//
// This uses [Retryer] with the default settings.
func Until(ctx context.Context, acceptor Acceptor) (bool, interface{}, error) {
	return (&Retryer{}).Until(ctx, acceptor)
}

// UntilDeadline creates a child context with the given deadline, and then invokes the above Until function.
func UntilDeadline(ctx context.Context, acceptor Acceptor, deadline time.Time) (bool, interface{}, error) {
	var cancel context.CancelFunc
	ctx, cancel = context.WithDeadline(ctx, deadline)
	b, data, err := Until(ctx, acceptor)
	cancel()
	return b, data, err
}

// UntilTimeout creates a child context with the given timeout, and then invokes the above Until function.
func UntilTimeout(ctx context.Context, acceptor Acceptor, timeout time.Duration) (bool, interface{}, error) {
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, timeout)
	b, data, err := Until(ctx, acceptor)
	cancel()
	return b, data, err
}
