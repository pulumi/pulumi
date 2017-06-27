// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package contract

import (
	"fmt"
)

// failfast logs and panics the process in a way that is friendly to debugging.
func failfast(msg string) {
	panic(fmt.Sprintf("fatal: %v", msg))
}
