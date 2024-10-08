// Copyright 2016-2022, Pulumi Corporation.
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

package httpstate

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

type tokenSourceCapability interface {
	GetToken(ctx context.Context) (string, error)
}

// tokenSource is a helper type that manages the renewal of the lease token for a managed update.
type tokenSource struct {
	clock    clockwork.Clock
	requests chan tokenRequest
	done     chan bool
}

var _ tokenSourceCapability = &tokenSource{}

type tokenRequest chan<- tokenResponse

type expiredTokenError struct {
	err error
}

func (e expiredTokenError) Error() string {
	return fmt.Sprintf("token expired: %v", e.err)
}

func (e expiredTokenError) Unwrap() error {
	return e.err
}

type tokenResponse struct {
	token string
	err   error
}

func newTokenSource(
	ctx context.Context,
	clock clockwork.Clock,
	initialToken string,
	initialTokenExpires time.Time,
	duration time.Duration,
	refreshToken func(
		ctx context.Context,
		duration time.Duration,
		currentToken string,
	) (string, time.Time, error),
) (*tokenSource, error) {
	requests, done := make(chan tokenRequest), make(chan bool)
	ts := &tokenSource{clock: clock, requests: requests, done: done}
	go ts.handleRequests(ctx, initialToken, initialTokenExpires, duration, refreshToken)
	return ts, nil
}

func (ts *tokenSource) handleRequests(
	ctx context.Context,
	initialToken string,
	initialTokenExpires time.Time,
	duration time.Duration,
	refreshToken func(
		ctx context.Context,
		duration time.Duration,
		currentToken string,
	) (string, time.Time, error),
) {
	renewTicker := ts.clock.NewTicker(duration / 8)
	defer renewTicker.Stop()

	state := struct {
		token   string    // most recently renewed token
		error   error     // non-nil indicates a terminal error state
		expires time.Time // assumed expiry of the token
	}{
		token:   initialToken,
		expires: initialTokenExpires,
	}

	renewUpdateLeaseIfStale := func() {
		if state.error != nil {
			return
		}

		now := ts.clock.Now()

		// We will renew the lease after 50% of the duration
		// has elapsed to allow time for retries.
		stale := now.Add(duration / 2).After(state.expires)
		if !stale {
			return
		}

		logging.V(9).Infof("trying to renew token. Current token expiring at: %v ", state.expires)

		newToken, newTokenExpires, err := refreshToken(ctx, duration, state.token)
		// Renewing might fail because of network issues, or because the token is no longer valid.
		// We only care about the latter, if it's just a network issue we should retry again.
		var expired expiredTokenError
		if errors.As(err, &expired) {
			logging.V(3).Infof("error renewing lease: %v", err)
			state.error = fmt.Errorf("renewing lease: %w", err)
			renewTicker.Stop()
		} else if err != nil {
			// If we failed to renew the lease, we will retry in the next cycle.
			logging.V(3).Infof("error renewing lease: %v", err)
		} else {
			logging.V(5).Infof("renewed lease. Next expiry: %v", newTokenExpires)
			state.token = newToken
			state.expires = newTokenExpires
		}
	}

	for {
		select {
		case <-renewTicker.Chan():
			renewUpdateLeaseIfStale()
		case c, ok := <-ts.requests:
			if !ok {
				close(ts.done)
				return
			}
			// If ticker has not kept up, block on
			// renewing rather than risking returning a
			// stale token.
			renewUpdateLeaseIfStale()
			if state.error == nil {
				c <- tokenResponse{token: state.token}
			} else {
				c <- tokenResponse{err: state.error}
			}
		}
	}
}

func (ts *tokenSource) GetToken(ctx context.Context) (string, error) {
	ch := make(chan tokenResponse)

	select {
	case ts.requests <- ch:
	case <-ctx.Done():
		return "", ctx.Err()
	}

	select {
	case resp := <-ch:
		return resp.token, resp.err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (ts *tokenSource) Close() {
	close(ts.requests)
	<-ts.done
}
