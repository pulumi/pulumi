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

package contract

import (
	"fmt"
	"io"

	"github.com/pulumi/pulumi/sdk/go/pulumi/util/logging"
)

const failMsg = "A failure has occurred"

// failfast logs and panics the process in a way that is friendly to debugging.
func failfast(msg string) {
	panic(fmt.Sprintf("fatal: %v", msg))
}

// Fail unconditionally abandons the process.
func Fail() {
	failfast(failMsg)
}

// Failf unconditionally abandons the process, formatting and logging the given message.
func Failf(msg string, args ...interface{}) {
	failfast(fmt.Sprintf("%v: %v", failMsg, fmt.Sprintf(msg, args...)))
}

const assertMsg = "An assertion has failed"

// Assert checks a condition and Fails if it is false.
func Assert(cond bool) {
	if !cond {
		failfast(assertMsg)
	}
}

// Assertf checks a condition and Failfs if it is false, formatting and logging the given message.
func Assertf(cond bool, msg string, args ...interface{}) {
	if !cond {
		failfast(fmt.Sprintf("%v: %v", assertMsg, fmt.Sprintf(msg, args...)))
	}
}

// IgnoreClose closes and ignores the returned error.  This makes defer closes easier.
func IgnoreClose(cr io.Closer) {
	err := cr.Close()
	IgnoreError(err)
}

// IgnoreError explicitly ignores an error.  This is similar to `_ = x`, but tells linters ignoring is intentional.
// This routine is specifically for ignoring errors which is potentially more risky, and so logs at a higher level.
func IgnoreError(err error) {
	// Log something at a verbose level just in case it helps to track down issues (e.g., an error that was
	// ignored that represents something even more egregious than the eventual failure mode).  If this truly matters, it
	// probably implies the ignore was not appropriate, but as a safeguard, logging seems useful.
	if err != nil {
		logging.V(3).Infof("Explicitly ignoring and discarding error: %v", err)
	}
}
// AssertNoError will Fail if the error is non-nil.
func AssertNoError(err error) {
	if err != nil {
		failfast(err.Error())
	}
}
