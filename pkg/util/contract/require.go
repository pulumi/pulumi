// Copyright 2016 Marapongo, Inc. All rights reserved.

package contract

import (
	"fmt"
)

const requireMsg = "A precondition has failed for %v"

// Require checks a precondition condition pertaining to a function parameter, and Fails if it is false.
func Require(cond bool, param string) {
	if !cond {
		failfast(fmt.Sprintf(requireMsg, param))
	}
}

// Requiref checks a precondition condition pertaining to a function parameter, and Failfs if it is false.
func Requiref(cond bool, param string, msg string, args ...interface{}) {
	if !cond {
		failfast(fmt.Sprintf("%v: %v", fmt.Sprintf(requireMsg, param), fmt.Sprintf(msg, args...)))
	}
}
