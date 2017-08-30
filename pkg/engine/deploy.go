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
	"github.com/pulumi/pulumi-fabric/pkg/util/contract"
)

type DeployOptions struct {
	Environment          string   // the environment we are deploying into
	Package              string   // the package we are deploying (or "" to use the default)
	Analyzers            []string // an optional set of analyzers to run as part of this deployment.
	Debug                bool     // true to enable resource debugging output.
	DryRun               bool     // true if we should just print the plan without performing it.
	ShowConfig           bool     // true to show the configuration variables being used.
	ShowReads            bool     // true to show the read-only steps in the plan.
	ShowReplacementSteps bool     // true to show the replacement steps in the plan.
	ShowSames            bool     // true to show the resources that aren't updated, in addition to those that are.
	Summary              bool     // true if we should only summarize resources and operations.
}

func (eng *Engine) Deploy(opts DeployOptions) error {
	info, err := eng.initEnvCmdName(tokens.QName(opts.Environment), opts.Package)
	if err != nil {
		return err
	}
	return eng.deployLatest(info, deployOptions{
		Debug:                opts.Debug,
		Destroy:              false,
		DryRun:               opts.DryRun,
		Analyzers:            opts.Analyzers,
		ShowConfig:           opts.ShowConfig,
		ShowReads:            opts.ShowReads,
		ShowReplacementSteps: opts.ShowReplacementSteps,
		ShowSames:            opts.ShowSames,
		Summary:              opts.Summary,
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
}

func (eng *Engine) deployLatest(info *envCmdInfo, opts deployOptions) error {
	result, err := eng.plan(info, opts)
	if err != nil {
		return err
	}
	if result != nil {
		defer contract.IgnoreClose(result)
		if opts.DryRun {
			// If a dry run, just print the plan, don't actually carry out the deployment.
			if err := eng.printPlan(result, opts); err != nil {
				return err
			}
		} else {
			// Otherwise, we will actually deploy the latest bits.
			var header bytes.Buffer
			printPrelude(&header, result, opts, false)
			header.WriteString(fmt.Sprintf("%vDeploying changes:%v\n", colors.SpecUnimportant, colors.Reset))
			fmt.Fprint(eng.Stdout, colors.Colorize(&header))

			// Create an object to track progress and perform the actual operations.
			start := time.Now()
			progress := newProgress(opts, eng)
			summary, _, _, err := result.Plan.Apply(progress)
			if err != nil && summary == nil {
				// Something went wrong, and we have no checkpoint to save.
				return err
			}
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

			// Now save the updated snapshot Notee that if a failure has occurred, the Apply routine above will
			// have returned a safe checkpoint.
			targ := result.Info.Target
			eng.saveEnv(targ, summary.Snap(), true /*overwrite*/)

			fmt.Fprint(eng.Stdout, colors.Colorize(&footer))
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
	Engine       *Engine
}

func newProgress(opts deployOptions, engine *Engine) *deployProgress {
	return &deployProgress{
		Steps:  0,
		Ops:    make(map[deploy.StepOp]int),
		Opts:   opts,
		Engine: engine,
	}
}

func (prog *deployProgress) Before(step deploy.Step) {
	if shouldShow(step, prog.Opts) {
		var b bytes.Buffer
		printStep(&b, step, prog.Opts.Summary, false, "")
		fmt.Fprint(prog.Engine.Stdout, colors.Colorize(&b))
	}
}

func (prog *deployProgress) After(step deploy.Step, status resource.Status, err error) {
	stepop := step.Op()
	if err != nil {
		// Issue a true, bonafide error.
		prog.Engine.Diag().Errorf(errors.ErrorPlanApplyFailed, err)

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
		fmt.Fprint(prog.Engine.Stdout, colors.Colorize(&b))
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
			fmt.Fprint(prog.Engine.Stdout, colors.Colorize(&b))
		}
	}
}
