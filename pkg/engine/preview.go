// Copyright 2017, Pulumi Corporation.  All rights reserved.

package engine

import (
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

type PreviewOptions struct {
	Package              string   // the package to compute the preview for
	Analyzers            []string // an optional set of analyzers to run as part of this deployment.
	Parallel             int      // the degree of parallelism for resource operations (<=1 for serial).
	ShowConfig           bool     // true to show the configuration variables being used.
	ShowReplacementSteps bool     // true to show the replacement steps in the plan.
	ShowSames            bool     // true to show the resources that aren't updated, in addition to those that are.
	Summary              bool     // true if we should only summarize resources and operations.
}

func (eng *Engine) Preview(stack tokens.QName, events chan<- Event, opts PreviewOptions) error {
	contract.Require(stack != tokens.QName(""), "stack")
	contract.Require(events != nil, "events")

	info, err := eng.planContextFromStack(stack, opts.Package)
	if err != nil {
		return err
	}

	return eng.previewLatest(info, deployOptions{
		Destroy:              false,
		DryRun:               true,
		Analyzers:            opts.Analyzers,
		Parallel:             opts.Parallel,
		ShowConfig:           opts.ShowConfig,
		ShowReplacementSteps: opts.ShowReplacementSteps,
		ShowSames:            opts.ShowSames,
		Summary:              opts.Summary,
		Events:               events,
		Diag: newEventSink(events, diag.FormatOptions{
			Colors: true,
		}),
	})
}

func (eng *Engine) previewLatest(info *planContext, opts deployOptions) error {
	result, err := eng.plan(info, opts)
	if err != nil {
		return err
	}
	if result != nil {
		defer contract.IgnoreClose(result)
		if err := eng.printPlan(result); err != nil {
			return err
		}
	}
	if !opts.Diag.Success() {
		// If any error occurred while walking the plan, be sure to let the developer know.  Otherwise,
		// although error messages may have spewed to the output, the final lines of the plan may look fine.
		return errors.New("One or more errors occurred during the creation of this preview")
	}
	return nil

}
