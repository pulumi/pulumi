// Copyright 2016 Marapongo, Inc. All rights reserved.

package contract

import (
	"github.com/golang/glog"
)

// failfast logs and panics the process in a way that is friendly to debugging.
func failfast(msg string) {
	glog.Fatal(msg)
}
