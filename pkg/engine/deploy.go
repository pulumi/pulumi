package engine

import (
	"bytes"
	"fmt"
	"time"

	"github.com/pulumi/pulumi-fabric/pkg/compiler/errors"
	"github.com/pulumi/pulumi-fabric/pkg/diag/colors"
	"github.com/pulumi/pulumi-fabric/pkg/resource"
	"github.com/pulumi/pulumi-fabric/pkg/resource/deploy"
	"github.com/pulumi/pulumi-fabric/pkg/tokens"
	"github.com/pulumi/pulumi-fabric/pkg/util/cmdutil"
	"github.com/pulumi/pulumi-fabric/pkg/util/contract"
)

type DeployOptions struct {
	Environment          string   // the environment we are deploying into
	Package              string   // the package we are deploying (or "" to use the default)
	Debug                bool     // true to enable resource debugging output.
	DryRun               bool     // true if we should just print the plan without performing it.
	Analyzers            []string // an optional set of analyzers to run as part of this deployment.
	ShowConfig           bool     // true to show the configuration variables being used.
	ShowReads            bool     // true to show the read-only steps in the plan.
	ShowReplacementSteps bool     // true to show the replacement steps in the plan.
	ShowSames            bool     // true to show the resources that aren't updated, in addition to those that are.
	Summary              bool     // true if we should only summarize resources and operations.
	Output               string   // the place to store the output, if any.
}

func Deploy(opts DeployOptions) error {
	info, err := initEnvCmdName(tokens.QName(opts.Environment), opts.Package)
	if err != nil {
		return err
	}
	return deployLatest(info, deployOptions{
		Debug:                opts.Debug,
		Destroy:              false,
		DryRun:               opts.DryRun,
		Analyzers:            opts.Analyzers,
		ShowConfig:           opts.ShowConfig,
		ShowReads:            opts.ShowReads,
		ShowReplacementSteps: opts.ShowReplacementSteps,
		ShowSames:            opts.ShowSames,
		Summary:              opts.Summary,
		Output:               opts.Output,
	})
}

type deployOptions struct {
	Debug                bool     // true to enable resource debugging output.
	Create               bool     // true if we are creating resources.
	Destroy              bool     // true if we are destroying the environment.
	DryRun               bool     // true if we should just print the plan without performing it.
	Analyzers            []string // an optional set of analyzers to run as part of this deployment.
	ShowConfig           bool     // true to show the configuration variables being used.
	ShowReads            bool     // true to show the read-only steps in the plan.
	ShowReplacementSteps bool     // true to show the replacement steps in the plan.
	ShowSames            bool     // true to show the resources that aren't updated, in addition to those that are.
	Summary              bool     // true if we should only summarize resources and operations.
	DOT                  bool     // true if we should print the DOT file for this plan.
	Output               string   // the place to store the output, if any.
}

func deployLatest(info *envCmdInfo, opts deployOptions) error {
	result, err := plan(info, opts)
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

			// Print a summary.
			var footer bytes.Buffer
			// Print out the total number of steps performed (and their kinds), the duration, and any summary info.
			if c := printChangeSummary(&footer, progress.Ops, false); c != 0 {
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
	if shouldShow(step, prog.Opts) {
		var b bytes.Buffer
		printStep(&b, step, prog.Opts.Summary, false, "")
		fmt.Print(colors.Colorize(&b))
	}
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
	} else {
		// Increment the counters.
		if step.Logical() {
			prog.Steps++
			prog.Ops[stepop]++
		}

		// Print out any output properties that got created as a result of this operation.
		if shouldShow(step, prog.Opts) && !prog.Opts.Summary {
			var b bytes.Buffer
			printResourceOutputProperties(&b, step, "")
			fmt.Print(colors.Colorize(&b))
		}
	}
}
