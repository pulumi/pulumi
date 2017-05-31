// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/pulumi/lumi/pkg/compiler/errors"
	"github.com/pulumi/lumi/pkg/diag/colors"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/cmdutil"
	"github.com/pulumi/lumi/pkg/util/contract"
)

func newDeployCmd() *cobra.Command {
	var analyzers []string
	var dryRun bool
	var env string
	var showConfig bool
	var showReplaceSteps bool
	var showUnchanged bool
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
			defer info.Close()
			deploy(cmd, info, deployOptions{
				Delete:           false,
				DryRun:           dryRun,
				Analyzers:        analyzers,
				ShowConfig:       showConfig,
				ShowReplaceSteps: showReplaceSteps,
				ShowUnchanged:    showUnchanged,
				Summary:          summary,
				Output:           output,
			})
			return nil
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
		&showReplaceSteps, "show-replace-steps", false,
		"Show detailed resource replacement creates and deletes; normally shows as a single step")
	cmd.PersistentFlags().BoolVar(
		&showUnchanged, "show-unchanged", false,
		"Show resources that needn't be updated because they haven't changed, alongside those that do")
	cmd.PersistentFlags().BoolVarP(
		&summary, "summary", "s", false,
		"Only display summarization of resources and plan operations")
	cmd.PersistentFlags().StringVarP(
		&output, "output", "o", "",
		"Serialize the resulting checkpoint to a specific file, instead of overwriting the existing one")

	return cmd
}

func deploy(cmd *cobra.Command, info *envCmdInfo, opts deployOptions) {
	if result := plan(cmd, info, opts); result != nil {
		// Now based on whether a dry run was specified, or not, either print or perform the planned operations.
		if opts.DryRun {
			// If no output file was requested, or "-", print to stdout; else write to that file.
			if opts.Output == "" || opts.Output == "-" {
				printPlan(info.Ctx.Diag, result, opts)
			} else {
				saveEnv(info.Env, result.New, opts.Output, true /*overwrite*/)
			}
		} else {
			// If show unchanged was requested, print them first, along with a header.
			var header bytes.Buffer
			printPrelude(&header, result, opts)
			header.WriteString(fmt.Sprintf("%vDeploying changes:%v\n", colors.SpecUnimportant, colors.Reset))
			fmt.Printf(colors.Colorize(&header))

			// Print a nice message if the update is an empty one.
			empty := checkEmpty(info.Ctx.Diag, result.Plan)

			// Create an object to track progress and perform the actual operations.
			start := time.Now()
			progress := newProgress(info.Ctx, opts.Summary)
			checkpoint, err, _, _ := result.Plan.Apply(progress)
			if err != nil {
				contract.Assert(!info.Ctx.Diag.Success()) // an error should have been emitted.
			}

			var summary bytes.Buffer
			if !empty {
				// Print out the total number of steps performed (and their kinds), the duration, and any summary info.
				printSummary(&summary, progress.Ops, opts.ShowReplaceSteps, false)
				summary.WriteString(fmt.Sprintf("%vDeployment duration: %v%v\n",
					colors.SpecUnimportant, time.Since(start), colors.Reset))
			}

			if progress.MaybeCorrupt {
				summary.WriteString(fmt.Sprintf(
					"%vA catastrophic error occurred; resources states may be unknown%v\n",
					colors.SpecAttention, colors.Reset))
			}

			// Now save the updated snapshot to the specified output file, if any, or the standard location otherwise.
			// Note that if a failure has occurred, the Apply routine above will have returned a safe checkpoint.
			env := result.Info.Env
			saveEnv(env, checkpoint, opts.Output, true /*overwrite*/)

			fmt.Printf(colors.Colorize(&summary))
		}
	}
}

type deployOptions struct {
	Create           bool     // true if we are creating resources.
	Delete           bool     // true if we are deleting resources.
	DryRun           bool     // true if we should just print the plan without performing it.
	Analyzers        []string // an optional set of analyzers to run as part of this deployment.
	ShowConfig       bool     // true to show the configuration variables being used.
	ShowReplaceSteps bool     // true to show the replacement steps in the plan.
	ShowUnchanged    bool     // true to show the resources that aren't updated, in addition to those that are.
	Summary          bool     // true if we should only summarize resources and operations.
	DOT              bool     // true if we should print the DOT file for this plan.
	Output           string   // the place to store the output, if any.
}

// deployProgress pretty-prints the plan application process as it goes.
type deployProgress struct {
	Ctx          *resource.Context
	Steps        int
	Ops          map[resource.StepOp]int
	MaybeCorrupt bool
	Summary      bool
}

func newProgress(ctx *resource.Context, summary bool) *deployProgress {
	return &deployProgress{
		Ctx:     ctx,
		Steps:   0,
		Ops:     make(map[resource.StepOp]int),
		Summary: summary,
	}
}

func (prog *deployProgress) Before(step resource.Step) {
	// Print the step.
	stepop := step.Op()
	stepnum := prog.Steps + 1

	var extra string
	if stepop == resource.OpReplaceCreate || stepop == resource.OpReplaceDelete {
		extra = " (part of a replacement change)"
	}

	var b bytes.Buffer
	b.WriteString(fmt.Sprintf("Applying step #%v [%v]%v\n", stepnum, stepop, extra))
	printStep(&b, step, prog.Summary, "")
	fmt.Printf(colors.Colorize(&b))
}

func (prog *deployProgress) After(step resource.Step, state resource.State, err error) {
	if err == nil {
		// Increment the counters.
		prog.Steps++
		prog.Ops[step.Op()]++

		// Print out any output properties that got created as a result of this operation.
		if step.Op() == resource.OpCreate {
			var b bytes.Buffer
			printResourceOutputProperties(&b, step, "")
			fmt.Printf(colors.Colorize(&b))
		}
	} else {
		// Issue a true, bonafide error.
		prog.Ctx.Diag.Errorf(errors.ErrorPlanApplyFailed, err)

		// Print the state of the resource; we don't issue the error, because the deploy above will do that.
		var b bytes.Buffer
		stepnum := prog.Steps + 1
		b.WriteString(fmt.Sprintf("Step #%v failed [%v]: ", stepnum, step.Op()))
		switch state {
		case resource.StateOK:
			b.WriteString(colors.SpecNote)
			b.WriteString("provider successfully recovered from this failure")
		case resource.StateUnknown:
			b.WriteString(colors.SpecAttention)
			b.WriteString("this failure was catastrophic and the provider cannot guarantee recovery")
			prog.MaybeCorrupt = true
		default:
			contract.Failf("Unrecognized resource state: %v", state)
		}
		b.WriteString(colors.Reset)
		b.WriteString("\n")
		fmt.Printf(colors.Colorize(&b))
	}
}
