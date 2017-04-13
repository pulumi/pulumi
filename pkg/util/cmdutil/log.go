// Copyright 2017 Pulumi, Inc. All rights reserved.

package cmdutil

import (
	"flag"
	"strconv"
)

// InitLogging ensures the glog library has been initialized with the given settings.
func InitLogging(logToStderr bool, verbose int) {
	// Ensure the glog library has been initialized, including calling flag.Parse beforehand.  Unfortunately,
	// this is the only way to control the way glog runs.  That includes poking around at flags below.
	flag.Parse()
	if logToStderr {
		flag.Lookup("logtostderr").Value.Set("true")
	}
	if verbose > 0 {
		flag.Lookup("v").Value.Set(strconv.Itoa(verbose))
	}
}
