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
	"fmt"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

// tokenSource is a helper type that manages the renewal of the lease token for a managed update.
type tokenSource struct {
	requests chan tokenRequest
	done     chan bool
}

type tokenRequest chan<- tokenResponse

type tokenResponse struct {
	token string
	err   error
}

func newTokenSource(
	ctx context.Context,
	initialToken string,
	duration time.Duration,
	refreshToken func(
		ctx context.Context,
		duration time.Duration,
		currentToken string,
	) (string, error),
) (*tokenSource, error) {
	requests, done := make(chan tokenRequest), make(chan bool)
	ts := &tokenSource{requests: requests, done: done}
	go ts.handleRequests(ctx, initialToken, duration, refreshToken)
	return ts, nil
}

func (ts *tokenSource) handleRequests(
	ctx context.Context,
	initialToken string,
	duration time.Duration,
	refreshToken func(
		ctx context.Context,
		duration time.Duration,
		currentToken string,
	) (string, error),
) {

	renewTicker := time.NewTicker(duration / 8)
	defer renewTicker.Stop()

	state := struct {
		token   string    // most recently renewed token
		error   error     // non-nil indicates a terminal error state
		expires time.Time // assumed expiry of the token
	}{
		token:   initialToken,
		expires: time.Now().Add(duration),
	}

	renewUpdateLeaseIfStale := func() {
		if state.error != nil {
			return
		}

		now := time.Now()

		// We will renew the lease after 50% of the duration
		// has elapsed to allow time for retries.
		stale := now.Add(duration / 2).After(state.expires)
		if !stale {
			return
		}

		newToken, err := refreshToken(ctx, duration, state.token)
		// If renew failed, all further GetToken requests will return this error.
		if err != nil {
			logging.V(3).Infof("error renewing lease: %v", err)
			state.error = fmt.Errorf("renewing lease: %w", err)
			renewTicker.Stop()
		} else {
			state.token = newToken
			state.expires = now.Add(duration)
		}
	}

	for {
		select {
		case <-renewTicker.C:
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

func (ts *tokenSource) GetToken() (string, error) {
	ch := make(chan tokenResponse)
	ts.requests <- ch
	resp := <-ch
	return resp.token, resp.err
}

func (ts *tokenSource) Close() {
	close(ts.requests)
	<-ts.done
}
