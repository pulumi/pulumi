// Copyright 2016 Marapongo, Inc. All rights reserved.

package util

import (
	"fmt"

	"github.com/golang/glog"
)

const assertFailure = "An assertion has failed"

func Assert(cond bool) {
	if !cond {
		glog.Fatal(assertFailure)
	}
}

func AssertM(cond bool, msg string) {
	if !cond {
		glog.Fatalf("%v: %v", assertFailure, msg)
	}
}

func AssertMF(cond bool, msg string, args ...interface{}) {
	if !cond {
		glog.Fatalf("%v: %v", assertFailure, fmt.Sprintf(msg, args...))
	}
}
