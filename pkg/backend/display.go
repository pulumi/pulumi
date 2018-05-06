// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package backend

import "github.com/pulumi/pulumi/pkg/diag/colors"

// DisplayOptions controls how the output of events are rendered
type DisplayOptions struct {
	Color                colors.Colorization // colorization to apply to events.
	ShowConfig           bool                // true if we should show configuration information.
	ShowReplacementSteps bool                // true to show the replacement steps in the plan.
	ShowSameResources    bool                // true to show the resources that aren't updated in addition to updates.
	SummaryDiff          bool                // If the diff display should be summarized
	IsInteractive        bool                // If we should display things interactively
	DiffDisplay          bool                // true if we should display things as a rich diff
	SuppressStackOutputs bool                // If the stack outputs should be not be shown
	Debug                bool
}
