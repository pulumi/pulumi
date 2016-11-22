// Copyright 2016 Marapongo, Inc. All rights reserved.

package util

import (
	"fmt"

	"github.com/golang/glog"
)

const failMsg = "A failure has occurred"

// Fail unconditionally abandons the process.
func Fail() {
	glog.Fatal(failMsg)
}

// FailM unconditionally abandons the process, logging the given message.
func FailM(msg string) {
	glog.Fatalf("%v: %v", failMsg, msg)
}

// FailMF unconditionally abandons the process, formatting and logging the given message.
func FailMF(msg string, args ...interface{}) {
	glog.Fatalf("%v: %v", failMsg, fmt.Sprintf(msg, args...))
}
