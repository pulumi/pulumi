// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package main

import (
	"bytes"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/pulumi/lumi/pkg/compiler/errors"
	"github.com/pulumi/lumi/pkg/diag"
	"github.com/pulumi/lumi/pkg/diag/colors"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/resource/deploy"
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/cmdutil"
	"github.com/pulumi/lumi/pkg/util/contract"
)

func newDeployCmd() *cobra.Command {
	var analyzers []string
	var dryRun bool
	var env string
	var showConfig bool
	var showReads bool
	var showReplaceDeletes bool
	var showSames bool
	var summary bool
	var output string
	var cmd = &cobra.Command{
		Use:     "deploy [<package>] [-- [<args>]]",
		Aliases: []string{"up", "update"},
		Short:   "Deploy resource updates, creations, and deletions to an environment",
		Long: "Deploy resource updates, creations, and deletions to an environment\n" +
			"\n" +
			"This command updates an existing environment whose state is represented by the\n" +
			"existing snapshot file.  The new desired state is computed by compiling and evaluating an\n" +
			"executable package, and extracting all resource allocations from its resulting object graph.\n" +
			"This graph is compared against the existing state to determine what operations must take\n" +
			"place to achieve the desired state.  This command results in a full snapshot of the\n" +
			"environment's new resource state, so that it may be updated incrementally again later.\n" +
			"\n" +
			"By default, the package to execute is loaded from the current directory.  Optionally, an\n" +
			"explicit path can be provided using the [package] argument.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			info, err := initEnvCmdName(tokens.QName(env), args)
			if err != nil {
				return err
			}
			return deployLatest(cmd, info, deployOptions{
				Destroy:            false,
				DryRun:             dryRun,
				Analyzers:          analyzers,
				ShowConfig:         showConfig,
				ShowReads:          showReads,
				ShowReplaceDeletes: showReplaceDeletes,
				ShowSames:          showSames,
				Summary:            summary,
				Output:             output,
			})
		}),
	}

	cmd.PersistentFlags().StringSliceVar(
		&analyzers, "analyzer", []string{},
		"Run one or more analyzers as part of this deployment")
	cmd.PersistentFlags().BoolVarP(
		&dryRun, "dry-run", "n", false,
		"Don't actually update resources, just print out the planned updates (synonym for plan)")
	cmd.PersistentFlags().StringVarP(
		&env, "env", "e", "",
		"Choose an environment other than the currently selected one")
	cmd.PersistentFlags().BoolVar(
		&showConfig, "show-config", false,
		"Show configuration keys and variables")
	cmd.PersistentFlags().BoolVar(
		&showReads, "show-reads", false,
		"Show resources that will be read, in addition to those that will be modified")
	cmd.PersistentFlags().BoolVar(
		&showReplaceDeletes, "show-replace-deletes", false,
		"Show detailed resource replacement creates and deletes; normally shows as a single step")
	cmd.PersistentFlags().BoolVar(
		&showSames, "show-sames", false,
		"Show resources that needn't be updated because they haven't changed, alongside those that do")
	cmd.PersistentFlags().BoolVarP(
		&summary, "summary", "s", false,
		"Only display summarization of resources and plan operations")
	cmd.PersistentFlags().StringVarP(
		&output, "output", "o", "",
		"Serialize the resulting checkpoint to a specific file, instead of overwriting the existing one")

	return cmd
}

type deployOptions struct {
	Create             bool     // true if we are creating resources.
	Destroy            bool     // true if we are destroying the environment.
	DryRun             bool     // true if we should just print the plan without performing it.
	Analyzers          []string // an optional set of analyzers to run as part of this deployment.
	ShowConfig         bool     // true to show the configuration variables being used.
	ShowReads          bool     // true to show the read-only steps in the plan.
	ShowReplaceDeletes bool     // true to show the replacement deletion steps in the plan.
	ShowSames          bool     // true to show the resources that aren't updated, in addition to those that are.
	Summary            bool     // true if we should only summarize resources and operations.
	DOT                bool     // true if we should print the DOT file for this plan.
	Output             string   // the place to store the output, if any.
}

func deployLatest(cmd *cobra.Command, info *envCmdInfo, opts deployOptions) error {
	result, err := plan(cmd, info, opts)
	if err != nil {
		return err
	}
	if result != nil {
		defer contract.IgnoreClose(result)
		if opts.DryRun {
			// If a dry run, just print the plan, don't actually carry out the deployment.
			if err := printPlan(result, opts); err != nil {
				return err
			}
		} else {
			// Otherwise, we will actually deploy the latest bits.
			var header bytes.Buffer
			printPrelude(&header, result, opts, false)
			header.WriteString(fmt.Sprintf("%vDeploying changes:%v\n", colors.SpecUnimportant, colors.Reset))
			fmt.Print(colors.Colorize(&header))

			// Create an object to track progress and perform the actual operations.
			start := time.Now()
			progress := newProgress(opts)
			summary, _, _, err := result.Plan.Apply(progress)
			contract.Assert(summary != nil)
			empty := (summary.Steps() == 0) // if no step is returned, it was empty.

			// Print a summary.
			var footer bytes.Buffer
			if empty {
				cmdutil.Diag().Infof(diag.Message("no resources need to be updated"))
			} else {
				// Print out the total number of steps performed (and their kinds), the duration, and any summary info.
				printSummary(&footer, progress.Ops, false)
				footer.WriteString(fmt.Sprintf("%vDeployment duration: %v%v\n",
					colors.SpecUnimportant, time.Since(start), colors.Reset))
			}

			if progress.MaybeCorrupt {
				footer.WriteString(fmt.Sprintf(
					"%vA catastrophic error occurred; resources states may be unknown%v\n",
					colors.SpecAttention, colors.Reset))
			}

			// Now save the updated snapshot to the specified output file, if any, or the standard location otherwise.
			// Note that if a failure has occurred, the Apply routine above will have returned a safe checkpoint.
			targ := result.Info.Target
			saveEnv(targ, summary.Snap(), opts.Output, true /*overwrite*/)

			fmt.Print(colors.Colorize(&footer))
			return err
		}
	}
	return nil
}

// deployProgress pretty-prints the plan application process as it goes.
type deployProgress struct {
	Steps        int
	Ops          map[deploy.StepOp]int
	MaybeCorrupt bool
	Opts         deployOptions
}

func newProgress(opts deployOptions) *deployProgress {
	return &deployProgress{
		Steps: 0,
		Ops:   make(map[deploy.StepOp]int),
		Opts:  opts,
	}
}

func (prog *deployProgress) Before(step deploy.Step) {
	stepop := step.Op()
	if stepop == deploy.OpSame {
		return
	}

	// Print the step.
	stepnum := prog.Steps + 1

	var extra string
	if stepop == deploy.OpReplace ||
		(stepop == deploy.OpDelete && step.(*deploy.DeleteStep).Replaced()) {
		extra = " (part of a replacement change)"
	}

	var b bytes.Buffer
	b.WriteString(fmt.Sprintf("Applying step #%v [%v]%v\n", stepnum, stepop, extra))
	printStep(&b, step, prog.Opts.Summary, false, "")
	fmt.Print(colors.Colorize(&b))
}

func (prog *deployProgress) After(step deploy.Step, status resource.Status, err error) {
	stepop := step.Op()
	if err != nil {
		// Issue a true, bonafide error.
		cmdutil.Diag().Errorf(errors.ErrorPlanApplyFailed, err)

		// Print the state of the resource; we don't issue the error, because the deploy above will do that.
		var b bytes.Buffer
		stepnum := prog.Steps + 1
		b.WriteString(fmt.Sprintf("Step #%v failed [%v]: ", stepnum, stepop))
		switch status {
		case resource.StatusOK:
			b.WriteString(colors.SpecNote)
			b.WriteString("provider successfully recovered from this failure")
		case resource.StatusUnknown:
			b.WriteString(colors.SpecAttention)
			b.WriteString("this failure was catastrophic and the provider cannot guarantee recovery")
			prog.MaybeCorrupt = true
		default:
			contract.Failf("Unrecognized resource state: %v", status)
		}
		b.WriteString(colors.Reset)
		b.WriteString("\n")
		fmt.Print(colors.Colorize(&b))
	} else if shouldTrack(step, prog.Opts) {
		// Increment the counters.
		prog.Steps++
		prog.Ops[stepop]++

		// Print out any output properties that got created as a result of this operation.
		if step.Op() == deploy.OpCreate {
			var b bytes.Buffer
			printResourceOutputProperties(&b, step, "")
			fmt.Print(colors.Colorize(&b))
		}
	}
}
