// Copyright 2023, Pulumi Corporation.
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

package client

import (
	"fmt"
	"net/http"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/httputil"
)

// retryPolicy defines the policy for retrying requests by httpClient.Do.
type retryPolicy int

const (
	// retryNone indicates that no retry should be attempted.
	retryNone retryPolicy = iota - 1

	// retryGetMethod indicates that only GET requests should be retried.
	//
	// This is the default retry policy.
	retryGetMethod // == 0

	// retryAllMethods indicates that all requests should be retried.
	retryAllMethods
)

func (p retryPolicy) String() string {
	switch p {
	case retryNone:
		return "none"
	case retryGetMethod:
		return "get"
	case retryAllMethods:
		return "all"
	default:
		return fmt.Sprintf("retryPolicy(%d)", p)
	}
}

func (p retryPolicy) shouldRetry(req *http.Request) bool {
	switch p {
	case retryNone:
		return false
	case retryGetMethod:
		return req.Method == http.MethodGet
	case retryAllMethods:
		return true
	default:
		contract.Failf("unknown retry policy: %v", p)
		return false // unreachable
	}
}

func doWithRetry(client *http.Client, req *http.Request, policy retryPolicy) (*http.Response, error) {
	if policy.shouldRetry(req) {
		// Wait 1s before retrying on failure. Then increase by 2x until the
		// maximum delay is reached. Stop after maxRetryCount requests have
		// been made.
		opts := httputil.RetryOpts{
			Delay:    ptr(time.Second),
			Backoff:  ptr(float64(2.0)),
			MaxDelay: ptr(30 * time.Second),

			MaxRetryCount: ptr(int(4)),
		}
		return httputil.DoWithRetryOpts(req, client, opts)
	}
	return client.Do(req)
}

func ptr[T any](v T) *T {
	return &v
}
