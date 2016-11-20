// Copyright 2016 Marapongo, Inc. All rights reserved.

package util

import (
	"fmt"

	"github.com/golang/glog"
)

const assertMsg = "An assertion has failed"

// Assert checks a condition and Fails if it is false.
func Assert(cond bool) {
	if !cond {
		glog.Fatal(assertMsg)
	}
}

// AssertM checks a condition and FailsMs if it is false, logging the given message.
func AssertM(cond bool, msg string) {
	if !cond {
		glog.Fatalf("%v: %v", assertMsg, msg)
	}
}

// AssertMF checks a condition and FailsMFs if it is false, formatting and logging the given message.
func AssertMF(cond bool, msg string, args ...interface{}) {
	if !cond {
		glog.Fatalf("%v: %v", assertMsg, fmt.Sprintf(msg, args...))
	}
}
