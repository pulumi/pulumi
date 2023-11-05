// Copyright 2016-2022, Pulumi Corporation.
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

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	survey "github.com/AlecAivazis/survey/v2"
	terminal "github.com/AlecAivazis/survey/v2/terminal"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func newRefreshCmd() *cobra.Command {
	var debug bool
	var expectNop bool
	var message string
	var execKind string
	var execAgent string
	var stackName string

	// Flags for remote operations.
	remoteArgs := RemoteArgs{}

	// Flags for engine.UpdateOptions.
	var jsonDisplay bool
	var diffDisplay bool
	var eventLogPath string
	var parallel int
	var showConfig bool
	var showReplacementSteps bool
	var showSames bool
	var skipPreview bool
	var suppressOutputs bool
	var suppressPermalink string
	var yes bool
	var targets *[]string

	// Flags for handling pending creates
	var skipPendingCreates bool
	var clearPendingCreates bool
	var importPendingCreates *[]string

	use, cmdArgs := "refresh", cmdutil.NoArgs
	if remoteSupported() {
		use, cmdArgs = "refresh [url]", cmdutil.MaximumNArgs(1)
	}

	cmd := &cobra.Command{
		Use:   use,
		Short: "Refresh the resources in a stack",
		Long: "Refresh the resources in a stack.\n" +
			"\n" +
			"This command compares the current stack's resource state with the state known to exist in\n" +
			"the actual cloud provider. Any such changes are adopted into the current stack. Note that if\n" +
			"the program text isn't updated accordingly, subsequent updates may still appear to be out of\n" +
			"sync with respect to the cloud provider's source of truth.\n" +
			"\n" +
			"The program to run is loaded from the project in the current directory. Use the `-C` or\n" +
			"`--cwd` flag to use a different directory.",
		Args: cmdArgs,
		Run: cmdutil.RunResultFunc(func(cmd *cobra.Command, args []string) result.Result {
			ctx := commandContext()

			// Remote implies we're skipping previews.
			if remoteArgs.remote {
				skipPreview = true
			}

			yes = yes || skipPreview || skipConfirmations()
			interactive := cmdutil.Interactive()
			if !interactive && !yes {
				return result.FromError(
					errors.New("--yes or --skip-preview must be passed in to proceed when running in non-interactive mode"))
			}

			opts, err := updateFlagsToOptions(interactive, skipPreview, yes)
			if err != nil {
				return result.FromError(err)
			}

			displayType := display.DisplayProgress
			if diffDisplay {
				displayType = display.DisplayDiff
			}

			opts.Display = display.Options{
				Color:                cmdutil.GetGlobalColorization(),
				ShowConfig:           showConfig,
				ShowReplacementSteps: showReplacementSteps,
				ShowSameResources:    showSames,
				SuppressOutputs:      suppressOutputs,
				IsInteractive:        interactive,
				Type:                 displayType,
				EventLogPath:         eventLogPath,
				Debug:                debug,
				JSONDisplay:          jsonDisplay,
			}

			// we only suppress permalinks if the user passes true. the default is an empty string
			// which we pass as 'false'
			if suppressPermalink == "true" {
				opts.Display.SuppressPermalink = true
			} else {
				opts.Display.SuppressPermalink = false
			}

			if remoteArgs.remote {
				if len(args) == 0 {
					return result.FromError(errors.New("must specify remote URL"))
				}

				err = validateUnsupportedRemoteFlags(expectNop, nil, false, "", jsonDisplay, nil,
					nil, "", showConfig, false, showReplacementSteps, showSames, false,
					suppressOutputs, "default", targets, nil, nil,
					false, "", stackConfigFile)
				if err != nil {
					return result.FromError(err)
				}

				return runDeployment(ctx, opts.Display, apitype.Refresh, stackName, args[0], remoteArgs)
			}

			filestateBackend, err := isFilestateBackend(opts.Display)
			if err != nil {
				return result.FromError(err)
			}

			// by default, we are going to suppress the permalink when using self-managed backends
			// this can be re-enabled by explicitly passing "false" to the `suppress-permalink` flag
			if suppressPermalink != "false" && filestateBackend {
				opts.Display.SuppressPermalink = true
			}

			s, err := requireStack(ctx, stackName, stackOfferNew, opts.Display)
			if err != nil {
				return result.FromError(err)
			}

			proj, root, err := readProject()
			if err != nil {
				return result.FromError(err)
			}

			ps, err := loadProjectStack(proj, s)
			if err != nil {
				return result.FromError(err)
			}

			m, err := getUpdateMetadata(message, root, execKind, execAgent, false, ps.Environment, cmd.Flags())
			if err != nil {
				return result.FromError(fmt.Errorf("gathering environment metadata: %w", err))
			}

			cfg, sm, err := getStackConfiguration(ctx, s, proj, nil)
			if err != nil {
				return result.FromError(fmt.Errorf("getting stack configuration: %w", err))
			}

			decrypter, err := sm.Decrypter()
			if err != nil {
				return result.FromError(fmt.Errorf("getting stack decrypter: %w", err))
			}
			encrypter, err := sm.Encrypter()
			if err != nil {
				return result.FromError(fmt.Errorf("getting stack encrypter: %w", err))
			}

			stackName := s.Ref().Name().String()
			configErr := workspace.ValidateStackConfigAndApplyProjectConfig(
				stackName,
				proj,
				cfg.Environment,
				cfg.Config,
				encrypter,
				decrypter)
			if configErr != nil {
				return result.FromError(fmt.Errorf("validating stack config: %w", configErr))
			}

			if skipPendingCreates && clearPendingCreates {
				return result.FromError(fmt.Errorf(
					"cannot set both --skip-pending-creates and --clear-pending-creates"))
			}

			// First we handle explicit create->imports we were given
			if importPendingCreates != nil && len(*importPendingCreates) > 0 {
				stderr := opts.Display.Stderr
				if stderr == nil {
					stderr = os.Stderr
				}
				if unused, err := pendingCreatesToImports(ctx, s, yes, opts.Display, *importPendingCreates); err != nil {
					return result.FromError(err)
				} else if len(unused) > 1 {
					fmt.Fprintf(stderr, "%s\n- \"%s\"\n", opts.Display.Color.Colorize(colors.Highlight(
						"warning: the following urns did not correspond to a pending create",
						"warning", colors.SpecWarning)),
						strings.Join(unused, "\"\n- \""))
				} else if len(unused) > 0 {
					fmt.Fprintf(stderr, "%s: \"%s\" did not correspond to a pending create\n",
						opts.Display.Color.Colorize(colors.Highlight("warning", "warning", colors.SpecWarning)),
						unused[0])
				}
			}

			snap, err := s.Snapshot(ctx, stack.DefaultSecretsProvider)
			if err != nil {
				return result.FromError(fmt.Errorf("getting snapshot: %w", err))
			}

			// We then allow the user to interactively handle remaining pending creates.
			if interactive && hasPendingCreates(snap) && !skipPendingCreates {
				if err := filterMapPendingCreates(ctx, s, opts.Display,
					yes, interactiveFixPendingCreate); err != nil {
					return result.FromError(err)
				}
			}

			// We remove remaining pending creates
			if clearPendingCreates && hasPendingCreates(snap) {
				// Remove all pending creates.
				removePendingCreates := func(op resource.Operation) (*resource.Operation, error) {
					return nil, nil
				}
				err := filterMapPendingCreates(ctx, s, opts.Display, yes, removePendingCreates)
				if err != nil {
					return result.FromError(err)
				}
			}

			targetUrns := []string{}
			targetUrns = append(targetUrns, *targets...)

			opts.Engine = engine.UpdateOptions{
				Parallel:                  parallel,
				Debug:                     debug,
				UseLegacyDiff:             useLegacyDiff(),
				DisableProviderPreview:    disableProviderPreview(),
				DisableResourceReferences: disableResourceReferences(),
				DisableOutputValues:       disableOutputValues(),
				Targets:                   deploy.NewUrnTargets(targetUrns),
				Experimental:              hasExperimentalCommands(),
			}

			changes, res := s.Refresh(ctx, backend.UpdateOperation{
				Proj:               proj,
				Root:               root,
				M:                  m,
				Opts:               opts,
				StackConfiguration: cfg,
				SecretsManager:     sm,
				SecretsProvider:    stack.DefaultSecretsProvider,
				Scopes:             backend.CancellationScopes,
			})

			switch {
			case res != nil && res.Error() == context.Canceled:
				return result.FromError(errors.New("refresh cancelled"))
			case res != nil:
				return PrintEngineResult(res)
			case expectNop && changes != nil && engine.HasChanges(changes):
				return result.FromError(errors.New("error: no changes were expected but changes occurred"))
			default:
				return nil
			}
		}),
	}

	cmd.PersistentFlags().BoolVarP(
		&debug, "debug", "d", false,
		"Print detailed debugging output during resource operations")
	cmd.PersistentFlags().BoolVar(
		&expectNop, "expect-no-changes", false,
		"Return an error if any changes occur during this update")
	cmd.PersistentFlags().StringVarP(
		&stackName, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.PersistentFlags().StringVar(
		&stackConfigFile, "config-file", "",
		"Use the configuration values in the specified file rather than detecting the file name")

	cmd.PersistentFlags().StringVarP(
		&message, "message", "m", "",
		"Optional message to associate with the update operation")

	targets = cmd.PersistentFlags().StringArrayP(
		"target", "t", []string{},
		"Specify a single resource URN to refresh. Multiple resource can be specified using: --target urn1 --target urn2")

	// Flags for engine.UpdateOptions.
	cmd.PersistentFlags().BoolVar(
		&diffDisplay, "diff", false,
		"Display operation as a rich diff showing the overall change")
	cmd.Flags().BoolVarP(
		&jsonDisplay, "json", "j", false,
		"Serialize the refresh diffs, operations, and overall output as JSON")
	cmd.PersistentFlags().IntVarP(
		&parallel, "parallel", "p", defaultParallel,
		"Allow P resource operations to run in parallel at once (1 for no parallelism). Defaults to unbounded.")
	cmd.PersistentFlags().BoolVar(
		&showReplacementSteps, "show-replacement-steps", false,
		"Show detailed resource replacement creates and deletes instead of a single step")
	cmd.PersistentFlags().BoolVar(
		&showSames, "show-sames", false,
		"Show resources that needn't be updated because they haven't changed, alongside those that do")
	cmd.PersistentFlags().BoolVarP(
		&skipPreview, "skip-preview", "f", false,
		"Do not calculate a preview before performing the refresh")
	cmd.PersistentFlags().BoolVar(
		&suppressOutputs, "suppress-outputs", false,
		"Suppress display of stack outputs (in case they contain sensitive values)")
	cmd.PersistentFlags().StringVar(
		&suppressPermalink, "suppress-permalink", "",
		"Suppress display of the state permalink")
	cmd.Flag("suppress-permalink").NoOptDefVal = "false"
	cmd.PersistentFlags().BoolVarP(
		&yes, "yes", "y", false,
		"Automatically approve and perform the refresh after previewing it")

	// Flags for pending creates
	cmd.PersistentFlags().BoolVar(
		&skipPendingCreates, "skip-pending-creates", false,
		"Skip importing pending creates in interactive mode")
	cmd.PersistentFlags().BoolVar(
		&clearPendingCreates, "clear-pending-creates", false,
		"Clear all pending creates, dropping them from the state")
	importPendingCreates = cmd.PersistentFlags().StringArray(
		"import-pending-creates", nil,
		"A list of form [[URN ID]...] describing the provider IDs of pending creates")

	// Remote flags
	remoteArgs.applyFlags(cmd)

	if hasDebugCommands() {
		cmd.PersistentFlags().StringVar(
			&eventLogPath, "event-log", "",
			"Log events to a file at this path")
	}

	// internal flags
	cmd.PersistentFlags().StringVar(&execKind, "exec-kind", "", "")
	// ignore err, only happens if flag does not exist
	_ = cmd.PersistentFlags().MarkHidden("exec-kind")
	cmd.PersistentFlags().StringVar(&execAgent, "exec-agent", "", "")
	// ignore err, only happens if flag does not exist
	_ = cmd.PersistentFlags().MarkHidden("exec-agent")

	return cmd
}

type editPendingOp = func(op resource.Operation) (*resource.Operation, error)

// filterMapPendingCreates applies f to each pending create. If f returns nil, then the op
// is deleted. Otherwise is is replaced by the returned op.
func filterMapPendingCreates(
	ctx context.Context, s backend.Stack, opts display.Options, yes bool, f editPendingOp,
) error {
	return totalStateEdit(ctx, s, yes, opts, func(opts display.Options, snap *deploy.Snapshot) error {
		var pending []resource.Operation
		for _, op := range snap.PendingOperations {
			if op.Resource == nil {
				return fmt.Errorf("found operation without resource")
			}
			if op.Type != resource.OperationTypeCreating {
				pending = append(pending, op)
				continue
			}
			op, err := f(op)
			if err != nil {
				return err
			}
			if op != nil {
				pending = append(pending, *op)
			}
		}
		snap.PendingOperations = pending
		return nil
	})
}

// Apply the CLI args from --import-pending-creates [[URN ID]...]. If an error was found,
// it is returned. The list of URNs that were not mapped to a pending create is also
// returned.
func pendingCreatesToImports(ctx context.Context, s backend.Stack, yes bool, opts display.Options,
	importToCreates []string,
) ([]string, error) {
	// A map from URN to ID
	if len(importToCreates)%2 != 0 {
		return nil, errors.New("each URN must be followed by an ID: found an odd number of entries")
	}
	alteredOps := make(map[string]string, len(importToCreates)/2)
	for i := 0; i < len(importToCreates); i += 2 {
		alteredOps[importToCreates[i]] = importToCreates[i+1]
	}
	err := filterMapPendingCreates(ctx, s, opts, yes, func(op resource.Operation) (*resource.Operation, error) {
		if id, ok := alteredOps[string(op.Resource.URN)]; ok {
			op.Resource.ID = resource.ID(id)
			op.Type = resource.OperationTypeImporting
			delete(alteredOps, string(op.Resource.URN))
			return &op, nil
		}
		return &op, nil
	})
	unusedKeys := make([]string, len(alteredOps))
	for k := range alteredOps {
		unusedKeys = append(unusedKeys, k)
	}
	return unusedKeys, err
}

func hasPendingCreates(snap *deploy.Snapshot) bool {
	if snap == nil {
		return false
	}
	for _, op := range snap.PendingOperations {
		if op.Type == resource.OperationTypeCreating {
			return true
		}
	}
	return false
}

func interactiveFixPendingCreate(op resource.Operation) (*resource.Operation, error) {
	for {
		option := ""
		options := []string{
			"clear (the CREATE failed; remove the pending CREATE)",
			"skip (do nothing)",
			"import (the CREATE succeeded; provide a resource ID and complete the CREATE operation)",
		}
		if err := survey.AskOne(&survey.Select{
			Message: fmt.Sprintf("Options for pending CREATE of %s", op.Resource.URN),
			Options: options,
		}, &option, nil); err != nil {
			return nil, fmt.Errorf("no option selected: %w", err)
		}

		var err error
		switch option {
		case options[0]:
			return nil, nil
		case options[1]:
			return &op, nil
		case options[2]:
			var id string
			err = survey.AskOne(&survey.Input{
				Message: "ID: ",
			}, &id, nil)
			if err == nil {
				op.Resource.ID = resource.ID(id)
				op.Type = resource.OperationTypeImporting
				return &op, nil
			}
		default:
			return nil, fmt.Errorf("unknown option: %q", option)
		}
		if errors.Is(err, terminal.InterruptErr) {
			continue
		}
		return nil, err
	}
}
