// Copyright 2016-2025, Pulumi Corporation.
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

package deploy

import "errors"

// The number of times to try an error hook before giving up.
const maxErrorHookRetries = 100

type maxErrorHookRetriesReachedError struct{}

func (e *maxErrorHookRetriesReachedError) Error() string {
	return "maximum number of error hook retries reached"
}

func (e *maxErrorHookRetriesReachedError) Is(target error) bool {
	_, ok := target.(*maxErrorHookRetriesReachedError)
	return ok
}

func isMaxErrorHookRetriesReached(err error) bool {
	return errors.Is(err, &maxErrorHookRetriesReachedError{})
}

// Retry an operation until it succeeds, encounters a non-retryable error, or hits the maximum number of retries.
func withRetries[T any](
	maxRetries int,
	operation func() (T, error),
	isRetryable func(result T, err error) bool,
	shouldRetry func(result T, failures []string) (bool, error),
) (T, error) {
	failures := []string{}

	for {
		result, err := operation()
		if err == nil {
			return result, nil
		}

		if !isRetryable(result, err) {
			return result, err
		}

		failures = append([]string{err.Error()}, failures...)
		if len(failures) >= maxRetries {
			return result, &maxErrorHookRetriesReachedError{}
		}

		retry, retryErr := shouldRetry(result, failures)
		if retryErr != nil {
			return result, retryErr
		}

		if !retry {
			return result, err
		}
	}
}
