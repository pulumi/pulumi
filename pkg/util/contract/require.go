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

// RequireM checks a precondition condition pertaining to a function parameter, and FailMs if it is false.
func RequireM(cond bool, param string, msg string) {
	if !cond {
		failfast(fmt.Sprintf("%v: %v", fmt.Sprintf(requireMsg, param), msg))
	}
}

// RequireMF checks a precondition condition pertaining to a function parameter, and FailMFs if it is false.
func RequireMF(cond bool, param string, msg string, args ...interface{}) {
	if !cond {
		failfast(fmt.Sprintf("%v: %v", fmt.Sprintf(requireMsg, param), fmt.Sprintf(msg, args...)))
	}
}
