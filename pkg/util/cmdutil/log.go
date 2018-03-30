// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package cmdutil

import (
	"flag"
	"strconv"

	"github.com/pulumi/pulumi/pkg/util/contract"
)

var LogToStderr = false // true if logging is being redirected to stderr.
var Verbose = 0         // >0 if verbose logging is enabled at a particular level.
var LogFlow = false     // true to flow logging settings to child processes.

// InitLogging ensures the glog library has been initialized with the given settings.
func InitLogging(logToStderr bool, verbose int, logFlow bool) {
	// Remember the settings in case someone inquires.
	LogToStderr = logToStderr
	Verbose = verbose
	LogFlow = logFlow

	// Ensure the glog library has been initialized, including calling flag.Parse beforehand.  Unfortunately,
	// this is the only way to control the way glog runs.  That includes poking around at flags below.
	flag.Parse()
	if logToStderr {
		err := flag.Lookup("logtostderr").Value.Set("true")
		contract.AssertNoError(err)
	}
	if verbose > 0 {
		err := flag.Lookup("v").Value.Set(strconv.Itoa(verbose))
		contract.AssertNoError(err)
	}
}
