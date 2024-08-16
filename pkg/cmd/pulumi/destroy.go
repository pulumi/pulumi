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
	"strconv"

	mapset "github.com/deckarep/golang-set/v2"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/graph"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

//nolint:lll
type DestroyArgs struct {
	Debug     bool   `argsShort:"d" argsUsage:"Print detailed debugging output during resource operations"`
	Remove    bool   `argsUsage:"Remove the stack and its config file after all resources in the stack have been deleted"`
	StackName string `args:"stack" argsShort:"s" argsUsage:"The name of the stack to operate on. Defaults to the current stack"`

	Message   string `argsShort:"m" argsUsage:"Optional message to associate with the destroy operation"`
	ExecKind  string
	ExecAgent string

	// Flags for remote operations.
	RemoteArgs RemoteArgs

	// Flags for engine.UpdateOptions.
	JSONDisplay          bool     `args:"json" argsShort:"j" argsUsage:"Serialize the destroy diffs, operations, and overall output as JSON"`
	DiffDisplay          bool     `args:"diff" argsUsage:"Display operation as a rich diff showing the overall change"`
	EventLogPath         string   `argsUsage:"Log events to a file at this path"`
	Parallel             int      `argsShort:"p" argsUsage:"Allow P resource operations to run in parallel at once (1 for no parallelism)"`
	PreviewOnly          bool     `argsUsage:"Only perform a preview of the destroy, don't perform the destroy itself"`
	Refresh              string   `argsShort:"r" argsUsage:"Refresh the state of the stack's resources before this update"`
	ShowConfig           bool     `argsUsage:"Show configuration keys and variables"`
	ShowReplacementSteps bool     `argsUsage:"Show detailed resource replacement creates and deletes instead of a single step"`
	ShowSames            bool     `argsUsage:"Show resources that don't need to be updated because they haven't changed, alongside those that do"`
	SkipPreview          bool     `argsShort:"f" argsUsage:"Do not perform a preview before performing the destroy"`
	SuppressOutputs      bool     `argsUsage:"Suppress display of stack outputs (in case they contain sensitive values)"`
	SuppressProgress     bool     `argsUsage:"Suppress display of periodic progress dots"`
	SuppressPermalink    string   `argsUsage:"Suppress display of the state permalink"`
	Yes                  bool     `argsShort:"y" argsUsage:"Automatically approve and perform the destroy after previewing it"`
	Targets              []string `args:"target" argsShort:"t" argsUsage:"Specify a single resource URN to destroy. All resources necessary to destroy this target will also be destroyed. Multiple resources can be specified using: --target urn1 --target urn2. Wildcards (*, **) are also supported" argsCommaSplit:"false"`
	TargetDependents     bool     `argsUsage:"Allows destroying of dependent targets discovered but not specified in --target list"`
	ExcludeProtected     bool     `argsUsage:"Do not destroy protected resources. Destroy all other resources."`
	ContinueOnError      bool     `argsUsage:"Continue to perform the destroy operation despite the occurrence of errors (can also be set with PULUMI_CONTINUE_ON_ERROR env var)"`
}

func newDestroyCmd(
	v *viper.Viper,
	parentPulumiCmd *cobra.Command,
) *cobra.Command {
	// TODO: hack/pulumirc
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
		Run: cmdutil.RunResultFunc(func(cmd *cobra.Command, cliArgs []string) result.Result {
			args := UnmarshalArgs[DestroyArgs](v, cmd)

			ctx := cmd.Context()

			// Remote implies we're skipping previews.
			if args.RemoteArgs.Remote {
				args.SkipPreview = true
			}

			// TODO: hack/pulumirc suspicous code
			args.Yes = args.Yes || args.SkipPreview || skipConfirmations()
			interactive := cmdutil.Interactive()
			if !interactive && !args.Yes && !args.PreviewOnly {
				return result.FromError(
					errors.New("--yes or --skip-preview or --preview-only " +
						"must be passed in to proceed when running in non-interactive mode"))
			}

			opts, err := updateFlagsToOptions(interactive, args.SkipPreview, args.Yes, args.PreviewOnly)
			if err != nil {
				return result.FromError(err)
			}

			displayType := display.DisplayProgress
			if args.DiffDisplay {
				displayType = display.DisplayDiff
			}

			opts.Display = display.Options{
				Color:                cmdutil.GetGlobalColorization(),
				ShowConfig:           args.ShowConfig,
				ShowReplacementSteps: args.ShowReplacementSteps,
				ShowSameResources:    args.ShowSames,
				SuppressOutputs:      args.SuppressOutputs,
				SuppressProgress:     args.SuppressProgress,
				IsInteractive:        interactive,
				Type:                 displayType,
				EventLogPath:         args.EventLogPath,
				Debug:                args.Debug,
				JSONDisplay:          args.JSONDisplay,
			}

			// we only suppress permalinks if the user passes true. the default is an empty string
			// which we pass as 'false'
			if args.SuppressPermalink == "true" {
				opts.Display.SuppressPermalink = true
			} else {
				opts.Display.SuppressPermalink = false
			}

			if args.RemoteArgs.Remote {
				err = validateUnsupportedRemoteFlags(false, nil, false, "", args.JSONDisplay, nil,
					nil, args.Refresh, args.ShowConfig, false, args.ShowReplacementSteps, args.ShowSames, false,
					args.SuppressOutputs, "default", args.Targets, nil, nil,
					args.TargetDependents, "", stackConfigFile)
				if err != nil {
					return result.FromError(err)
				}

				var url string
				if len(cliArgs) > 0 {
					url = cliArgs[0]
				}

				if errResult := validateRemoteDeploymentFlags(url, args.RemoteArgs); errResult != nil {
					return errResult
				}

				return runDeployment(ctx, cmd, opts.Display, apitype.Destroy, args.StackName, url, args.RemoteArgs)
			}

			isDIYBackend, err := isDIYBackend(opts.Display)
			if err != nil {
				return result.FromError(err)
			}

			// by default, we are going to suppress the permalink when using DIY backends
			// this can be re-enabled by explicitly passing "false" to the `suppress-permalink` flag
			if args.SuppressPermalink != "false" && isDIYBackend {
				opts.Display.SuppressPermalink = true
			}

			s, err := requireStack(ctx, args.StackName, stackLoadOnly, opts.Display)
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

			m, err := getUpdateMetadata(args.Message, root, args.ExecKind, args.ExecAgent, false, cmd.Flags())
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
			if args.StackName != "" {
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

			refreshOption, err := getRefreshOption(proj, args.Refresh)
			if err != nil {
				return result.FromError(err)
			}

			if len(args.Targets) > 0 && args.ExcludeProtected {
				return result.FromError(errors.New("You cannot specify --target and --exclude-protected"))
			}

			var protectedCount int
			targetUrns := args.Targets
			if args.ExcludeProtected {
				contract.Assertf(len(targetUrns) == 0, "Expected no target URNs, got %d", len(targetUrns))
				targetUrns, protectedCount, err = handleExcludeProtected(ctx, s)
				if err != nil {
					return result.FromError(err)
				} else if protectedCount > 0 && len(targetUrns) == 0 {
					if !args.JSONDisplay {
						fmt.Printf("There were no unprotected resources to destroy. There are still %d"+
							" protected resources associated with this stack.\n", protectedCount)
					}
					// We need to return now. Otherwise the update will conclude
					// we tried to destroy everything and error for trying to
					// destroy a protected resource.
					return nil
				}
			}

			opts.Engine = engine.UpdateOptions{
				Parallel:                  args.Parallel,
				Debug:                     args.Debug,
				Refresh:                   refreshOption,
				Targets:                   deploy.NewUrnTargets(targetUrns),
				TargetDependents:          args.TargetDependents,
				UseLegacyDiff:             useLegacyDiff(),
				UseLegacyRefreshDiff:      useLegacyRefreshDiff(),
				DisableProviderPreview:    disableProviderPreview(),
				DisableResourceReferences: disableResourceReferences(),
				DisableOutputValues:       disableOutputValues(),
				Experimental:              hasExperimentalCommands(),
				ContinueOnError:           args.ContinueOnError,
			}

			_, res := s.Destroy(ctx, backend.UpdateOperation{
				Proj:               proj,
				Root:               root,
				M:                  m,
				Opts:               opts,
				StackConfiguration: cfg,
				SecretsManager:     sm,
				SecretsProvider:    stack.DefaultSecretsProvider,
				Scopes:             backend.CancellationScopes,
			})

			if res == nil && protectedCount > 0 && !args.JSONDisplay {
				fmt.Printf("All unprotected resources were destroyed. There are still %d protected resources"+
					" associated with this stack.\n", protectedCount)
			} else if res == nil && len(args.Targets) == 0 {
				if !args.JSONDisplay && !args.Remove && !args.PreviewOnly {
					fmt.Printf("The resources in the stack have been deleted, but the history and configuration "+
						"associated with the stack are still maintained. \nIf you want to remove the stack "+
						"completely, run `pulumi stack rm %s`.\n", s.Ref())
				} else if args.Remove {
					_, err = s.Remove(ctx, false)
					if err != nil {
						return result.FromError(err)
					}
					// Remove also the stack config file.
					if _, path, err := workspace.DetectProjectStackPath(s.Ref().Name().Q()); err == nil {
						if err = os.Remove(path); err != nil && !os.IsNotExist(err) {
							return result.FromError(err)
						} else if !args.JSONDisplay {
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

	parentPulumiCmd.AddCommand(cmd)
	BindFlags[DestroyArgs](v, cmd)

	// TODO hack/pulumirc stackConfigFile here

	// TODO hack/pulumirc we deleted flags_test.go as part of this work because it
	// fundamentally doesn't work any more due to how we now bind arguments. We
	// should re-add it, or rather add tests for each command and its argument
	// reading.

	// TODO hack/pulumirc does PULUMI_CONTINUE_ON_ERROR still work?
	// Will: no, it doesn't. no non-tag default values do. We probably need a
	// better way of doing this.
	cmd.PersistentFlags().Lookup("continue-on-error").DefValue = strconv.FormatBool(env.ContinueOnError.Value())
	cmd.PersistentFlags().Lookup("parallel").DefValue = strconv.Itoa(defaultParallel)
	cmd.PersistentFlags().Lookup("refresh").NoOptDefVal = "true"
	cmd.Flag("suppress-permalink").NoOptDefVal = "false"

	// TODO hack/pulumirc remote flags
	// args.RemoteArgs.applyFlags(cmd)

	// internal flags
	// ignore err, only happens if flag does not exist
	_ = cmd.PersistentFlags().MarkHidden("exec-kind")
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
