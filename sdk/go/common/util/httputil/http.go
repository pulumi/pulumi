// Copyright 2016, Pulumi Corporation.
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

package httputil

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/retry"
)

const maxRetryAfterDelay = 30 * time.Second

// shouldRetryHeader lets the server override the caller's retry policy on
// error responses: "true" forces a retry, "false" forbids one. Follows the
// x-should-retry convention used by Stainless-generated SDKs.
const shouldRetryHeader = "X-Pulumi-Should-Retry"

// retryAfterHeaderDelay returns the uncapped delay from a Retry-After header
// (RFC 9110 delay-seconds or HTTP-date); ok is false if absent or malformed.
// A past HTTP-date is a valid zero delay.
func retryAfterHeaderDelay(res *http.Response, now time.Time) (time.Duration, bool) {
	header := res.Header.Get("Retry-After")
	if header == "" {
		return 0, false
	}
	var delay time.Duration
	if seconds, err := strconv.Atoi(header); err == nil {
		// delay-seconds is 1*DIGIT per the RFC; negative is malformed.
		if seconds < 0 {
			return 0, false
		}
		delay = time.Duration(seconds) * time.Second
	} else if date, err := http.ParseTime(header); err == nil {
		delay = date.Sub(now)
	} else {
		return 0, false
	}
	return max(delay, 0), true
}

// retryAfterDelay returns the capped delay from a Retry-After header, or 0 if
// absent or unparseable.
func retryAfterDelay(res *http.Response, now time.Time) time.Duration {
	delay, ok := retryAfterHeaderDelay(res, now)
	if !ok {
		return 0
	}
	return min(delay, maxRetryAfterDelay)
}

// RetryOpts defines options to configure the retry behavior.
// Leave nil for defaults.
type RetryOpts struct {
	// These fields map directly to util.Acceptor.
	Delay    *time.Duration
	Backoff  *float64
	MaxDelay *time.Duration

	MaxRetryCount *int
	// HandshakeTimeoutsOnly indicates whether we should only be retrying timeouts that occur during the TLS handshake.
	// These timeouts are safe to retry even on POST requests, since we know the actual request hasn't been sent yet.
	HandshakeTimeoutsOnly bool

	// OnRetryWait, if set, is called before honoring a server-directed wait
	// (Retry-After on a 429 or 503 response).
	OnRetryWait func(delay time.Duration, res *http.Response)
}

// DoWithRetry calls client.Do, and in the case of an error, retries the operation again after a slight delay.
// Uses the default retry delays, starting at 100ms and ramping up to ~1.3s.
func DoWithRetry(req *http.Request, client *http.Client) (*http.Response, error) {
	var opts RetryOpts
	return doWithRetry(req, client, opts)
}

// DoWithRetryOpts calls client.Do, but retrying 500s (even for POSTs). Using the provided delays.
func DoWithRetryOpts(req *http.Request, client *http.Client, opts RetryOpts) (*http.Response, error) {
	return doWithRetry(req, client, opts)
}

func doWithRetry(req *http.Request, client *http.Client, opts RetryOpts) (*http.Response, error) {
	contract.Assertf(req.ContentLength == 0 || req.GetBody != nil,
		"Retryable request must have no body or rewindable body")

	inRange := func(test, lower, upper int) bool {
		return lower <= test && test <= upper
	}

	// maxRetryCount is the number of times to try an http request before
	// giving up an returning the last error.
	maxRetryCount := 5
	if opts.MaxRetryCount != nil {
		maxRetryCount = *opts.MaxRetryCount
	}

	// Maintenance rounds (503 + Retry-After) don't count against the retry budget.
	maintenanceRounds := 0

	acceptor := retry.Acceptor{
		// If the opts field is nil, retry.Until will provide defaults.
		Delay:    opts.Delay,
		Backoff:  opts.Backoff,
		MaxDelay: opts.MaxDelay,

		Accept: func(try int, _ time.Duration) (bool, any, error) {
			attempts := try - maintenanceRounds
			if try > 0 && req.GetBody != nil {
				// Reset request body, if present, for retries.
				rc, bodyErr := req.GetBody()
				if bodyErr != nil {
					return false, nil, bodyErr
				}
				req.Body = rc
			}

			res, resErr := client.Do(req)

			serverSaysRetry := false
			if resErr == nil && res.StatusCode >= 400 {
				switch res.Header.Get(shouldRetryHeader) {
				case "true":
					serverSaysRetry = true
				case "false":
					return true, res, nil
				}
			}

			if opts.HandshakeTimeoutsOnly && !serverSaysRetry {
				if resErr != nil && strings.Contains(resErr.Error(), "net/http: TLS handshake timeout") {
					// If we have a handshake timeout, we can retry the request.
					return false, nil, nil
				}
				return true, res, resErr
			}

			// A 503 with Retry-After signals planned unavailability: honor the delay
			// uncapped and keep retrying for as long as the server keeps signaling.
			if resErr == nil && res.StatusCode == http.StatusServiceUnavailable {
				if delay, ok := retryAfterHeaderDelay(res, time.Now()); ok {
					maintenanceRounds++
					contract.IgnoreClose(res.Body)
					if delay > 0 {
						if opts.OnRetryWait != nil {
							opts.OnRetryWait(delay, res)
						}
						select {
						case <-req.Context().Done():
							return true, nil, req.Context().Err()
						case <-time.After(delay):
						}
					}
					return false, nil, nil
				}
			}

			// 429s are retried like 5xxs, but honor the server's Retry-After (capped)
			// before the regular backoff.
			if resErr == nil && res.StatusCode == http.StatusTooManyRequests && attempts < maxRetryCount-1 {
				delay := retryAfterDelay(res, time.Now())
				contract.IgnoreClose(res.Body)
				if delay > 0 {
					if opts.OnRetryWait != nil {
						opts.OnRetryWait(delay, res)
					}
					select {
					case <-req.Context().Done():
						return true, nil, req.Context().Err()
					case <-time.After(delay):
					}
				}
				return false, nil, nil
			}

			if resErr == nil && !inRange(res.StatusCode, 500, 599) && !serverSaysRetry {
				return true, res, nil
			}

			if attempts >= (maxRetryCount - 1) {
				return true, res, resErr
			}

			// Close the response body, if present, since our caller can't.
			if resErr == nil {
				contract.IgnoreClose(res.Body)
			}
			return false, nil, nil
		},
	}
	_, res, err := retry.Until(context.Background(), acceptor)
	if err != nil {
		return nil, err
	}

	return res.(*http.Response), nil
}

// GetWithRetry issues a GET request with the given client, and in the case of an error, retries the operation again
// after a slight delay.
func GetWithRetry(url string, client *http.Client) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	return DoWithRetry(req, client)
}
