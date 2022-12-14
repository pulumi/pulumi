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

package httputil

import (
	"context"
	"net/http"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/retry"
)

// RetryOpts defines options to configure the retry behavior.
// Leave nil for defaults.
type RetryOpts struct {
	// These fields map directly to util.Acceptor.
	Delay    *time.Duration
	Backoff  *float64
	MaxDelay *time.Duration

	MaxRetryCount *int
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

	acceptor := retry.Acceptor{
		// If the opts field is nil, retry.Until will provide defaults.
		Delay:    opts.Delay,
		Backoff:  opts.Backoff,
		MaxDelay: opts.MaxDelay,

		Accept: func(try int, _ time.Duration) (bool, interface{}, error) {
			if try > 0 && req.GetBody != nil {
				// Reset request body, if present, for retries.
				rc, bodyErr := req.GetBody()
				if bodyErr != nil {
					return false, nil, bodyErr
				}
				req.Body = rc
			}

			res, resErr := client.Do(req)
			if resErr == nil && !inRange(res.StatusCode, 500, 599) {
				return true, res, nil
			}
			if try >= (maxRetryCount - 1) {
				return true, res, resErr
			}

			// Close the response body, if present, since our caller can't.
			if resErr == nil {
				contract.IgnoreError(res.Body.Close())
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
