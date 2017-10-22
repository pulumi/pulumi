// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

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

// AssertNoError will Fail if the error is non-nil.
func AssertNoError(err error) {
	if err != nil {
		failfast(err.Error())
	}
}

// AssertNoErrorf will Fail if the error is non-nil, adding the additional log message.
func AssertNoErrorf(err error, msg string, args ...interface{}) {
	if err != nil {
		failfast(fmt.Sprintf("error %v: %v. source error: %v", assertMsg, fmt.Sprintf(msg, args...), err))
	}
}
