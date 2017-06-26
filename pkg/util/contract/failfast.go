// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package contract

import (
	"flag"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/golang/glog"
)

// failfast logs and panics the process in a way that is friendly to debugging.
func failfast(msg string) {
	v := flag.Lookup("logtostderr").Value
	if g, isgettable := v.(flag.Getter); isgettable {
		if enabled := g.Get().(bool); enabled {
			// Print the stack to stderr anytime glog verbose logging is enabled, since glog won't.
			if _, err := fmt.Fprintf(os.Stderr, "fatal: %v\n", msg); err != nil {
				glog.Infof("Printing fatal error failed with error: %v", err)
			}
			debug.PrintStack()
		}
	}
	glog.Fatal(msg)
}
