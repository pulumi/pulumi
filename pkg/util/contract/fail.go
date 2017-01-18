// Copyright 2016 Marapongo, Inc. All rights reserved.

package contract

import (
	"fmt"
)

const failMsg = "A failure has occurred"

// Fail unconditionally abandons the process.
func Fail() {
	failfast(failMsg)
}

// Failf unconditionally abandons the process, formatting and logging the given message.
func Failf(msg string, args ...interface{}) {
	failfast(fmt.Sprintf("%v: %v", failMsg, fmt.Sprintf(msg, args...)))
}
