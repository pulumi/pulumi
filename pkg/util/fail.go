// Copyright 2016 Marapongo, Inc. All rights reserved.

package util

import (
	"fmt"

	"github.com/golang/glog"
)

const failMsg = "A failure has occurred"

// Fail unconditionally abandons the process.
func Fail() {
	failfast(failMsg)
}

// FailM unconditionally abandons the process, logging the given message.
func FailM(msg string) {
	failfast(fmt.Sprintf("%v: %v", failMsg, msg))
}

// FailMF unconditionally abandons the process, formatting and logging the given message.
func FailMF(msg string, args ...interface{}) {
	failfast(fmt.Sprintf("%v: %v", failMsg, fmt.Sprintf(msg, args...)))
}

// failfast logs and panics the process in a way that is friendly to debugging.
func failfast(msg string) {
	glog.Fatal(msg)
}
