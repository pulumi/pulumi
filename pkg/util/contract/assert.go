// Copyright 2016 Marapongo, Inc. All rights reserved.

package contract

import (
	"fmt"
)

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
