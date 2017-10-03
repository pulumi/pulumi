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
	Debug                bool     // true to enable resource debugging output.
	Parallel             int      // the degree of parallelism for resource operations (<=1 for serial).
	ShowConfig           bool     // true to show the configuration variables being used.
	ShowReplacementSteps bool     // true to show the replacement steps in the plan.
	ShowSames            bool     // true to show the resources that aren't updated, in addition to those that are.
	Summary              bool     // true if we should only summarize resources and operations.
}

func (eng *Engine) Preview(environment tokens.QName, opts PreviewOptions) error {
	contract.Require(environment != tokens.QName(""), "environment")

	// Initialize the diagnostics logger with the right stuff.
	eng.InitDiag(diag.FormatOptions{
		Colors: true,
		Debug:  opts.Debug,
	})

	info, err := eng.planContextFromEnvironment(environment, opts.Package)
	if err != nil {
		return err
	}
	deployOpts := deployOptions{
		Debug:                opts.Debug,
		Destroy:              false,
		DryRun:               true,
		Analyzers:            opts.Analyzers,
		Parallel:             opts.Parallel,
		ShowConfig:           opts.ShowConfig,
		ShowReplacementSteps: opts.ShowReplacementSteps,
		ShowSames:            opts.ShowSames,
		Summary:              opts.Summary,
	}
	result, err := eng.plan(info, deployOpts)
	if err != nil {
		return err
	}
	if result != nil {
		defer contract.IgnoreClose(result)
		if err := eng.printPlan(result); err != nil {
			return err
		}
	}
	if !eng.Diag().Success() {
		// If any error occurred while walking the plan, be sure to let the developer know.  Otherwise,
		// although error messages may have spewed to the output, the final lines of the plan may look fine.
		return errors.New("One or more errors occurred during the creation of this preview")
	}
	return nil
}
