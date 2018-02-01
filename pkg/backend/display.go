// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package backend

import "github.com/pulumi/pulumi/pkg/diag/colors"

// DisplayOptions controls how the output of events are rendered
type DisplayOptions struct {
	Color colors.Colorization // colorization to apply to events
}
