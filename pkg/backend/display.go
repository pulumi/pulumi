// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package backend

import "github.com/pulumi/pulumi/pkg/diag/colors"

// DisplayOptions controls how the output of events are rendered
type DisplayOptions struct {
	Color                colors.Colorization // colorization to apply to events.
	ShowConfig           bool                // true if we should show configuration information.
	ShowReplacementSteps bool                // true to show the replacement steps in the plan.
	ShowSames            bool                // true to show the resources that aren't updated in addition to updates.
	Summary              bool                // true if we should only summarize resources and operations.
}
