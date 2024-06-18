// Copyright 2016-2021, Pulumi Corporation.
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

	mapset "github.com/deckarep/golang-set/v2"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/graph"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func newDestroyCmd() *cobra.Command {
	var opts *Options

	var remove bool
	var stackName string

	var message string
	var execKind string
	var execAgent string

	// Flags for remote operations.
	remoteArgs := RemoteArgs{}

	var diffDisplay bool
	var parallel int
	var previewOnly bool
	var refresh string
	var skipPreview bool
	var yes bool
	var targets *[]string
	var targetDependents bool
	var excludeProtected bool
	var continueOnError bool

	use, cmdArgs := "destroy", cmdutil.NoArgs
	if remoteSupported() {
		use, cmdArgs = "destroy [url]", cmdutil.MaximumNArgs(1)
	}

	cmd := &cobra.Command{
		Use:        use,
		Aliases:    []string{"down", "dn"},
		SuggestFor: []string{"delete", "kill", "remove", "rm", "stop"},
		Short:      "Destroy all existing resources in the stack",
		Long: "Destroy all existing resources in the stack, but not the stack itself\n" +
			"\n" +
			"Deletes all the resources in the selected stack.  The current state is\n" +
			"loaded from the associated state file in the workspace.  After running to completion,\n" +
			"all of this stack's resources and associated state are deleted.\n" +
			"\n" +
			"The stack itself is not deleted. Use `pulumi stack rm` or the \n" +
			"`--remove` flag to delete the stack and its config file.\n" +
			"\n" +
			"Warning: this command is generally irreversible and should be used with great care.",
		Args: cmdArgs,
		Run: cmdutil.RunResultFunc(func(cmd *cobra.Command, args []string) result.Result {
			ctx := cmd.Context()

			// Remote implies we're skipping previews.
			if remoteArgs.remote {
				skipPreview = true
			}

			yes = yes || skipPreview || skipConfirmations()
			interactive := opts.Display.IsInteractive
			if !interactive && !yes && !previewOnly {
				return result.FromError(
					errors.New("--yes or --skip-preview or --preview-only " +
						"must be passed in to proceed when running in non-interactive mode"))
			}

			updateOpts, err := updateFlagsToOptions(interactive, skipPreview, yes, previewOnly)
			if err != nil {
				return result.FromError(err)
			}

			updateOpts.Display = opts.AsDisplayOptions()

			if remoteArgs.remote {
				err = validateUnsupportedRemoteFlags(
					false,
					nil,
					false,
					"",
					opts.Display.JSONDisplay,
					nil,
					nil,
					refresh,
					opts.Display.ShowConfig,
					false,
					opts.Display.ShowReplacementSteps,
					opts.Display.ShowSameResources,
					false,
					opts.Display.SuppressOutputs,
					"default",
					targets,
					nil,
					nil,
					targetDependents,
					"",
					stackConfigFile,
				)
				if err != nil {
					return result.FromError(err)
				}

				var url string
				if len(args) > 0 {
					url = args[0]
				}

				return runDeployment(ctx, updateOpts.Display, apitype.Destroy, stackName, url, remoteArgs)
			}

			isDIYBackend, err := isDIYBackend()
			if err != nil {
				return result.FromError(err)
			}

			// by default, we are going to suppress the permalink when using DIY backends
			// this can be re-enabled by explicitly passing "false" to the `suppress-permalink` flag
			if opts.Display.SuppressPermalink != "false" && isDIYBackend {
				updateOpts.Display.SuppressPermalink = true
			}

			s, err := requireStack(ctx, stackName, stackLoadOnly, updateOpts.Display)
			if err != nil {
				return result.FromError(err)
			}

			proj, root, err := readProject()
			if err != nil && errors.Is(err, workspace.ErrProjectNotFound) {
				logging.Warningf("failed to find current Pulumi project, continuing with an empty project"+
					"using stack %v from backend %v", s.Ref().Name(), s.Backend().Name())
				projectName, has := s.Ref().Project()
				if !has {
					// If the stack doesn't have a project name (legacy diy) then leave this blank, as
					// we used to.
					projectName = ""
				}
				proj = &workspace.Project{
					Name: tokens.PackageName(projectName),
				}
				root = ""
			} else if err != nil {
				return result.FromError(err)
			}

			m, err := getUpdateMetadata(message, root, execKind, execAgent, false, cmd.Flags())
			if err != nil {
				return result.FromError(fmt.Errorf("gathering environment metadata: %w", err))
			}

			snap, err := s.Snapshot(ctx, stack.DefaultSecretsProvider)
			if err != nil {
				return result.FromError(err)
			}

			// Use the current snapshot secrets manager, if there is one, as the fallback secrets manager.
			var defaultSecretsManager secrets.Manager
			if snap != nil {
				defaultSecretsManager = snap.SecretsManager
			}

			getConfig := getStackConfiguration
			if stackName != "" {
				// `pulumi destroy --stack <stack>` can be run outside of the project directory.
				// The config may be missing, fallback on the latest configuration in the backend.
				getConfig = getStackConfigurationOrLatest
			}
			cfg, sm, err := getConfig(ctx, s, proj, defaultSecretsManager)
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
			configError := workspace.ValidateStackConfigAndApplyProjectConfig(
				ctx,
				stackName,
				proj,
				cfg.Environment,
				cfg.Config,
				encrypter,
				decrypter)
			if configError != nil {
				return result.FromError(fmt.Errorf("validating stack config: %w", configError))
			}

			refreshOption, err := getRefreshOption(proj, refresh)
			if err != nil {
				return result.FromError(err)
			}

			if len(*targets) > 0 && excludeProtected {
				return result.FromError(errors.New("You cannot specify --target and --exclude-protected"))
			}

			var protectedCount int
			targetUrns := *targets
			if excludeProtected {
				contract.Assertf(len(targetUrns) == 0, "Expected no target URNs, got %d", len(targetUrns))
				targetUrns, protectedCount, err = handleExcludeProtected(ctx, s)
				if err != nil {
					return result.FromError(err)
				} else if protectedCount > 0 && len(targetUrns) == 0 {
					if !opts.Display.JSONDisplay {
						fmt.Printf("There were no unprotected resources to destroy. There are still %d"+
							" protected resources associated with this stack.\n", protectedCount)
					}
					// We need to return now. Otherwise the update will conclude
					// we tried to destroy everything and error for trying to
					// destroy a protected resource.
					return nil
				}
			}

			updateOpts.Engine = engine.UpdateOptions{
				Parallel:                  parallel,
				Debug:                     opts.Display.Debug,
				Refresh:                   refreshOption,
				Targets:                   deploy.NewUrnTargets(targetUrns),
				TargetDependents:          targetDependents,
				UseLegacyDiff:             useLegacyDiff(),
				UseLegacyRefreshDiff:      useLegacyRefreshDiff(),
				DisableProviderPreview:    disableProviderPreview(),
				DisableResourceReferences: disableResourceReferences(),
				DisableOutputValues:       disableOutputValues(),
				Experimental:              hasExperimentalCommands(),
				ContinueOnError:           continueOnError,
			}

			_, res := s.Destroy(ctx, backend.UpdateOperation{
				Proj:               proj,
				Root:               root,
				M:                  m,
				Opts:               updateOpts,
				StackConfiguration: cfg,
				SecretsManager:     sm,
				SecretsProvider:    stack.DefaultSecretsProvider,
				Scopes:             backend.CancellationScopes,
			})

			if res == nil && protectedCount > 0 && !opts.Display.JSONDisplay {
				fmt.Printf("All unprotected resources were destroyed. There are still %d protected resources"+
					" associated with this stack.\n", protectedCount)
			} else if res == nil && len(*targets) == 0 {
				if !opts.Display.JSONDisplay && !remove && !previewOnly {
					fmt.Printf("The resources in the stack have been deleted, but the history and configuration "+
						"associated with the stack are still maintained. \nIf you want to remove the stack "+
						"completely, run `pulumi stack rm %s`.\n", s.Ref())
				} else if remove {
					_, err = s.Remove(ctx, false)
					if err != nil {
						return result.FromError(err)
					}
					// Remove also the stack config file.
					if _, path, err := workspace.DetectProjectStackPath(s.Ref().Name().Q()); err == nil {
						if err = os.Remove(path); err != nil && !os.IsNotExist(err) {
							return result.FromError(err)
						} else if !opts.Display.JSONDisplay {
							fmt.Printf("The resources in the stack have been deleted, and the history and " +
								"configuration removed.\n")
						}
					}
				}
			} else if res != nil && res.Error() == context.Canceled {
				return result.FromError(errors.New("destroy cancelled"))
			}
			return PrintEngineResult(res)
		}),
	}

	optsBuilder := NewOptionsBuilder("destroy", cmd).
		WithDisplayDebug().
		WithDisplayJSON().
		WithDisplayShowConfig().
		WithDisplayShowDiff().
		WithDisplayShowReplacementSteps().
		WithDisplayShowSameResources().
		WithDisplaySuppressOutputs().
		WithDisplaySuppressProgress().
		WithDisplaySuppressPermalink()

	if hasDebugCommands() {
		optsBuilder = optsBuilder.WithDisplayEventLogPath()
	}

	opts = optsBuilder.Build()

	cmd.PersistentFlags().BoolVar(
		&remove, "remove", false,
		"Remove the stack and its config file after all resources in the stack have been deleted")
	cmd.PersistentFlags().StringVarP(
		&stackName, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.PersistentFlags().StringVar(
		&stackConfigFile, "config-file", "",
		"Use the configuration values in the specified file rather than detecting the file name")
	cmd.PersistentFlags().StringVarP(
		&message, "message", "m", "",
		"Optional message to associate with the destroy operation")

	targets = cmd.PersistentFlags().StringArrayP(
		"target", "t", []string{},
		"Specify a single resource URN to destroy. All resources necessary to destroy this target will also be destroyed."+
			" Multiple resources can be specified using: --target urn1 --target urn2."+
			" Wildcards (*, **) are also supported")
	cmd.PersistentFlags().BoolVar(
		&targetDependents, "target-dependents", false,
		"Allows destroying of dependent targets discovered but not specified in --target list")
	cmd.PersistentFlags().BoolVar(&excludeProtected, "exclude-protected", false, "Do not destroy protected resources."+
		" Destroy all other resources.")

	// Flags for engine.UpdateOptions.
	cmd.PersistentFlags().BoolVar(
		&diffDisplay, "diff", false,
		"Display operation as a rich diff showing the overall change")
	cmd.PersistentFlags().IntVarP(
		&parallel, "parallel", "p", defaultParallel,
		"Allow P resource operations to run in parallel at once (1 for no parallelism).")
	cmd.PersistentFlags().BoolVar(
		&previewOnly, "preview-only", false,
		"Only show a preview of the destroy, but don't perform the destroy itself")
	cmd.PersistentFlags().StringVarP(
		&refresh, "refresh", "r", "",
		"Refresh the state of the stack's resources before this update")
	cmd.PersistentFlags().Lookup("refresh").NoOptDefVal = "true"
	cmd.PersistentFlags().BoolVarP(
		&skipPreview, "skip-preview", "f", false,
		"Do not calculate a preview before performing the destroy")
	cmd.PersistentFlags().BoolVar(
		&continueOnError, "continue-on-error", false,
		"Continue to perform the destroy operation despite the occurrence of errors")

	cmd.PersistentFlags().BoolVarP(
		&yes, "yes", "y", false,
		"Automatically approve and perform the destroy after previewing it")

	// Remote flags
	remoteArgs.applyFlags(cmd)

	// internal flags
	cmd.PersistentFlags().StringVar(&execKind, "exec-kind", "", "")
	// ignore err, only happens if flag does not exist
	_ = cmd.PersistentFlags().MarkHidden("exec-kind")
	cmd.PersistentFlags().StringVar(&execAgent, "exec-agent", "", "")
	// ignore err, only happens if flag does not exist
	_ = cmd.PersistentFlags().MarkHidden("exec-agent")

	return cmd
}

// separateProtected returns a list or unprotected and protected resources respectively. This allows
// us to safely destroy all resources in the unprotected list without invalidating any resource in
// the protected list. Protection is contravarient: A < B where A: Protected => B: Protected, A < B
// where B: Protected !=> A: Protected.
//
// A
// B: Parent = A
// C: Parent = A, Protect = True
// D: Parent = C
//
// -->
//
// Unprotected: B, D
// Protected: A, C
//
// We rely on the fact that `resources` is topologically sorted with respect to its dependencies.
// This function understands that providers live outside this topological sort.
func separateProtected(resources []*resource.State) (
	/*unprotected*/ []*resource.State /*protected*/, []*resource.State,
) {
	dg := graph.NewDependencyGraph(resources)
	transitiveProtected := mapset.NewSet[*resource.State]()
	for _, r := range resources {
		if r.Protect {
			rProtected := dg.TransitiveDependenciesOf(r)
			rProtected.Add(r)
			transitiveProtected = transitiveProtected.Union(rProtected)
		}
	}
	allResources := mapset.NewSet(resources...)
	return allResources.Difference(transitiveProtected).ToSlice(), transitiveProtected.ToSlice()
}

// Returns the number of protected resources that remain. Appends all unprotected resources to `targetUrns`.
func handleExcludeProtected(ctx context.Context, s backend.Stack) ([]string, int, error) {
	// Get snapshot
	snapshot, err := s.Snapshot(ctx, stack.DefaultSecretsProvider)
	if err != nil {
		return nil, 0, err
	} else if snapshot == nil {
		return nil, 0, errors.New("Failed to find the stack snapshot. Are you in a stack?")
	}
	unprotected, protected := separateProtected(snapshot.Resources)
	targetUrns := make([]string, len(unprotected))
	for i, r := range unprotected {
		targetUrns[i] = string(r.URN)
	}
	return targetUrns, len(protected), nil
}
