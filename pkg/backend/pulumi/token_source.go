// Copyright 2016-2020, Pulumi Corporation.
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

package pulumi

import (
	"context"
	"time"

	"github.com/pulumi/pulumi/pkg/v2/backend/pulumi/client"
)

type tokenRequest chan<- tokenResponse

type tokenResponse struct {
	token string
	err   error
}

// tokenSource is a helper type that manages the renewal of the lease token for a managed update.
type tokenSource struct {
	requests chan tokenRequest
	done     chan bool
}

func newTokenSource(ctx context.Context, token string, client *client.Client, update client.UpdateIdentifier,
	duration time.Duration) (*tokenSource, error) {

	// Perform an initial lease renewal.
	newToken, err := client.RenewUpdateLease(ctx, update, token, duration)
	if err != nil {
		return nil, err
	}

	requests, done := make(chan tokenRequest), make(chan bool)
	go func() {
		// We will renew the lease after 50% of the duration has elapsed to allow more time for retries.
		ticker := time.NewTicker(duration / 2)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				newToken, err = client.RenewUpdateLease(ctx, update, token, duration)
				if err != nil {
					ticker.Stop()
				} else {
					token = newToken
				}

			case c, ok := <-requests:
				if !ok {
					close(done)
					return
				}

				resp := tokenResponse{err: err}
				if err == nil {
					resp.token = token
				}
				c <- resp
			}
		}
	}()

	return &tokenSource{requests: requests, done: done}, nil
}

func (ts *tokenSource) Close() {
	close(ts.requests)
	<-ts.done
}

func (ts *tokenSource) GetToken() (string, error) {
	ch := make(chan tokenResponse)
	ts.requests <- ch
	resp := <-ch
	return resp.token, resp.err
}
