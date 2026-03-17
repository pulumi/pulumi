// Copyright 2016-2026, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package policy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/secrets"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

func newPolicyCheckCmd() *cobra.Command {
	var policyCheckCmd policyCheckCmd
	var stack string
	var policyPackPaths []string
	var policyPackConfigPaths []string

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check the current stack state against policy packs",
		Long: "Check the current stack state against policy packs.\n" +
			"\n" +
			"This command evaluates policy packs against the current stack's state without\n" +
			"running the program. It loads the stack's snapshot from the backend and runs\n" +
			"both per-resource and stack-wide policy checks.\n" +
			"\n" +
			"By default, it uses the policy packs configured for the stack's organization.\n" +
			"Use --policy-pack to specify local policy pack paths instead.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return policyCheckCmd.Run(ctx, stack, policyPackPaths, policyPackConfigPaths)
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"The name of the stack to check. Defaults to the current stack")
	cmd.PersistentFlags().StringArrayVar(
		&policyPackPaths, "policy-pack", nil,
		"Run checks from a local policy pack at the given path. May be specified multiple times")
	cmd.PersistentFlags().StringArrayVar(
		&policyPackConfigPaths, "policy-pack-config", nil,
		"Path to JSON file containing the config for the policy pack of the corresponding --policy-pack argument")

	return cmd
}

type policyCheckCmd struct {
	diag   diag.Sink
	stdout io.Writer // defaults to os.Stdout
	stderr io.Writer // defaults to os.Stderr

	// requireStack is a function that returns the stack to operate on.
	// If nil, the default implementation is used.
	requireStack func(ctx context.Context, stackName string) (backend.Stack, error)

	// checkFunc allows injecting engine.Check for testing.
	checkFunc func(
		ctx context.Context,
		u engine.UpdateInfo,
		opts engine.UpdateOptions,
		snap *deploy.Snapshot,
		events chan<- engine.Event,
	) (engine.CheckResult, error)
}

func (cmd *policyCheckCmd) Run(
	ctx context.Context,
	stackName string,
	policyPackPaths []string,
	policyPackConfigPaths []string,
) error {
	if cmd.diag == nil {
		cmd.diag = cmdutil.Diag()
	}
	if cmd.stdout == nil {
		cmd.stdout = os.Stdout
	}
	if cmd.stderr == nil {
		cmd.stderr = os.Stderr
	}
	if cmd.checkFunc == nil {
		cmd.checkFunc = engine.Check
	}

	// Validate that --policy-pack-config and --policy-pack counts match.
	if len(policyPackConfigPaths) > 0 && len(policyPackConfigPaths) != len(policyPackPaths) {
		return errors.New("--policy-pack-config must be specified for each --policy-pack")
	}

	if cmd.requireStack == nil {
		cmd.requireStack = func(ctx context.Context, stackName string) (backend.Stack, error) {
			displayOpts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}
			return cmdStack.RequireStack(ctx, cmd.diag, pkgWorkspace.Instance,
				cmdBackend.DefaultLoginManager, stackName, cmdStack.LoadOnly, displayOpts)
		}
	}

	// Get the stack.
	s, err := cmd.requireStack(ctx, stackName)
	if err != nil {
		if stackName == "" && errors.Is(err, workspace.ErrProjectNotFound) {
			return errors.New("could not find a Pulumi project in the current working directory; " +
				"please specify a stack using the --stack flag.")
		}
		return err
	}

	// Build update options with policy packs.
	var opts engine.UpdateOptions
	if len(policyPackPaths) > 0 {
		opts.LocalPolicyPacks = engine.MakeLocalPolicyPacks(policyPackPaths, policyPackConfigPaths)
	} else {
		// Fetch stack's configured policy packs.
		policyPacks, err := s.Backend().GetStackPolicyPacks(ctx, s.Ref())
		if err != nil {
			return fmt.Errorf("getting stack policy packs: %w", err)
		}
		requiredPolicies := make([]engine.RequiredPolicy, len(policyPacks))
		for i, p := range policyPacks {
			requiredPolicies[i] = p
		}
		opts.RequiredPolicies = requiredPolicies
	}

	if len(opts.LocalPolicyPacks) == 0 && len(opts.RequiredPolicies) == 0 {
		fmt.Fprintf(cmd.stderr, "No policy packs to check for stack %s\n", s.Ref().String())
		return nil
	}

	// Load the stack's snapshot.
	snap, err := s.Snapshot(ctx, secrets.DefaultProvider)
	if err != nil {
		return fmt.Errorf("loading stack snapshot: %w", err)
	}
	if snap == nil || len(snap.Resources) == 0 {
		fmt.Fprintf(cmd.stderr, "Stack %s has no resources to check\n", s.Ref().String())
		return nil
	}

	// Set up event channel and collect results.
	events := make(chan engine.Event)
	type checkOutput struct {
		result engine.CheckResult
		err    error
	}
	done := make(chan checkOutput)

	go func() {
		result, err := cmd.checkFunc(ctx, engine.UpdateInfo{}, opts, snap, events)
		// Close the events channel after Check returns so the event loop below terminates.
		close(events)
		done <- checkOutput{result, err}
	}()

	// Process events as they come in.
	color := cmdutil.GetGlobalColorization()
	var mandatoryCount, advisoryCount int
	for e := range events {
		switch payload := e.Payload().(type) {
		case engine.PolicyViolationEventPayload:
			msg := color.Colorize(payload.Prefix + payload.Message)
			fmt.Fprint(cmd.stdout, msg)
			switch payload.EnforcementLevel {
			case apitype.Mandatory:
				mandatoryCount++
			case apitype.Advisory:
				advisoryCount++
			}
		}
	}

	output := <-done
	if output.err != nil {
		return fmt.Errorf("policy check failed: %w", output.err)
	}
	result := output.result

	// Print summary.
	fmt.Fprintln(cmd.stdout)
	if result.Passed {
		if advisoryCount > 0 {
			fmt.Fprintf(cmd.stdout, "Policy check passed with %d advisory %s\n",
				advisoryCount, pluralize("violation", advisoryCount))
		} else {
			fmt.Fprintf(cmd.stdout, "Policy check passed\n")
		}
	} else {
		fmt.Fprintf(cmd.stdout, "Policy check failed: %d mandatory %s",
			result.MandatoryViolations, pluralize("violation", result.MandatoryViolations))
		if advisoryCount > 0 {
			fmt.Fprintf(cmd.stdout, ", %d advisory %s",
				advisoryCount, pluralize("violation", advisoryCount))
		}
		fmt.Fprintln(cmd.stdout)
		return errors.New("policy check failed")
	}

	return nil
}

func pluralize(word string, count int) string {
	if count == 1 {
		return word
	}
	return word + "s"
}
