// Copyright 2016-2023, Pulumi Corporation.
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
	"bytes"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/promise"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

//nolint:lll
type PreviewArgs struct {
	Debug                  bool   `argsShort:"d" argsUsage:"Print detailed debugging output during resource operations"`
	ExpectNoChanges        bool   `argsUsage:"Return an error if any changes are proposed by this preview"`
	Message                string `argsShort:"m" argsUsage:"Optional message to associate with the preview operation"`
	ExecKind               string
	ExecAgent              string
	Stack                  string   `argsShort:"s" argsUsage:"The name of the stack to operate on. Defaults to the current stack"`
	ConfigArray            []string `args:"config" argsCommaSplit:"false" argsShort:"c" argsUsage:"Config to use during the preview and save to the stack config file"`
	ConfigPath             bool     `argsUsage:"Config keys contain a path to a property in a map or list to set"`
	Client                 string   `argsUsage:"The address of an existing language runtime host to connect to"`
	PlanFilePath           string   `args:"save-plan" argsUsage:"[EXPERIMENTAL] Save the operations proposed by the preview to a plan file at the given path"`
	ImportFilePath         string   `args:"import-file" argsUsage:"Save any creates seen during the preview into an import file to use with 'pulumi import'"`
	ShowSecrets            bool     "argsUsage:\"Emit secrets in plaintext in the plan file. Defaults to `false`\""
	JSON                   bool     `args:"json" argsShort:"j" argsUsage:"Serialize the preview diffs, operations, and overall output as JSON"`
	PolicyPackPaths        []string `args:"policy-pack" argsUsage:"Run one or more policy packs as part of this update"`
	PolicyPackConfigPaths  []string "args:\"policy-pack-config\" argsUsage:\"Path to JSON file containing the config for the policy pack of the corresponding \\\"--policy-pack\\\" flag\""
	DisplayDiff            bool     `args:"diff" argsUsage:"Display operation as a rich diff showing the overall change"`
	EventLogPath           string   `args:"event-log" argsUsage:"Log events to a file at this path"`
	Parallel               int      `argsShort:"p" argsUsage:"Allow P resource operations to run in parallel at once (1 for no parallelism)."`
	Refresh                string   `argsShort:"r" argsUsage:"Refresh the state of the stack's resources before this update"`
	ShowConfig             bool     `argsUsage:"Show configuration keys and variables"`
	ShowPolicyRemediations bool     `argsUsage:"Show per-resource policy remediation details instead of a summary"`
	ShowReplacementSteps   bool     `argsUsage:"Show detailed resource replacement creates and deletes instead of a single step"`
	ShowSames              bool     `argsUsage:"Show resources that needn't be updated because they haven't changed, alongside those that do"`
	ShowReads              bool     `argsUsage:"Show resources that are being read in, alongside those being managed directly in the stack"`
	SuppressOutputs        bool     `argsUsage:"Suppress display of stack outputs (in case they contain sensitive values)"`
	SuppressProgress       bool     `argsUsage:"Suppress display of periodic progress dots"`
	SuppressPermalink      string   `argsUsage:"Suppress display of the state permalink"`
	Targets                []string `args:"target" argsCommaSplit:"false" argsShort:"t" argsUsage:"Specify a single resource URN to update. Other resources will not be updated. Multiple resources can be specified using --target urn1 --target urn2"`
	Replaces               []string `args:"replace" argsCommaSplit:"false" argsUsage:"Specify resources to replace. Multiple resources can be specified using --replace urn1 --replace urn2"`
	TargetReplaces         []string `args:"target-replace" argsCommmaSplit:"false" argsUsage:"Specify a single resource URN to replace. Other resources will not be updated. Shorthand for --target urn --replace urn."`
	TargetDependents       bool     `argsUsage:"Allows updating of dependent targets discovered but not specified in --target list"`
}

func newPreviewCmd(
	v *viper.Viper,
	parentPulumiCmd *cobra.Command,
) *cobra.Command {
	remoteArgs := RemoteArgs{}

	use, cmdArgs := "preview", cmdutil.NoArgs
	if remoteSupported() {
		use, cmdArgs = "preview [url]", cmdutil.MaximumNArgs(1)
	}

	cmd := &cobra.Command{
		Use:        use,
		Aliases:    []string{"pre"},
		SuggestFor: []string{"build", "plan"},
		Short:      "Show a preview of updates to a stack's resources",
		Long: "Show a preview of updates a stack's resources.\n" +
			"\n" +
			"This command displays a preview of the updates to an existing stack whose state is\n" +
			"represented by an existing state file. The new desired state is computed by running\n" +
			"a Pulumi program, and extracting all resource allocations from its resulting object graph.\n" +
			"These allocations are then compared against the existing state to determine what\n" +
			"operations must take place to achieve the desired state. No changes to the stack will\n" +
			"actually take place.\n" +
			"\n" +
			"The program to run is loaded from the project in the current directory. Use the `-C` or\n" +
			"`--cwd` flag to use a different directory.",
		Args: cmdArgs,
		Run: cmdutil.RunResultFunc(func(cmd *cobra.Command, cmdArgs []string) result.Result {
			args := UnmarshalArgs[PreviewArgs](v, cmd)

			ctx := cmd.Context()
			displayType := display.DisplayProgress
			if args.DisplayDiff {
				displayType = display.DisplayDiff
			}

			displayOpts := display.Options{
				Color:                  cmdutil.GetGlobalColorization(),
				ShowConfig:             args.ShowConfig,
				ShowPolicyRemediations: args.ShowPolicyRemediations,
				ShowReplacementSteps:   args.ShowReplacementSteps,
				ShowSameResources:      args.ShowSames,
				ShowReads:              args.ShowReads,
				SuppressOutputs:        args.SuppressOutputs,
				SuppressProgress:       args.SuppressProgress,
				IsInteractive:          cmdutil.Interactive(),
				Type:                   displayType,
				JSONDisplay:            args.JSON,
				EventLogPath:           args.EventLogPath,
				Debug:                  args.Debug,
			}

			// we only suppress permalinks if the user passes true. the default is an empty string
			// which we pass as 'false'
			if args.SuppressPermalink == "true" {
				displayOpts.SuppressPermalink = true
			} else {
				displayOpts.SuppressPermalink = false
			}

			if remoteArgs.Remote {
				err := validateUnsupportedRemoteFlags(
					args.ExpectNoChanges,
					args.ConfigArray,
					args.ConfigPath,
					args.Client,
					args.JSON,
					args.PolicyPackPaths,
					args.PolicyPackConfigPaths,
					args.Refresh,
					args.ShowConfig,
					args.ShowPolicyRemediations,
					args.ShowReplacementSteps,
					args.ShowSames,
					args.ShowReads,
					args.SuppressOutputs,
					"default",
					args.Targets,
					args.Replaces,
					args.TargetReplaces,
					args.TargetDependents,
					args.PlanFilePath,
					stackConfigFile,
				)
				if err != nil {
					return result.FromError(err)
				}

				var url string
				if len(cmdArgs) > 0 {
					url = cmdArgs[0]
				}

				if errResult := validateRemoteDeploymentFlags(url, remoteArgs); errResult != nil {
					return errResult
				}

				return runDeployment(ctx, cmd, displayOpts, apitype.Preview, args.Stack, url, remoteArgs)
			}

			isDIYBackend, err := isDIYBackend(displayOpts)
			if err != nil {
				return result.FromError(err)
			}

			// by default, we are going to suppress the permalink when using DIY backends
			// this can be re-enabled by explicitly passing "false" to the `suppress-permalink` flag
			if args.SuppressPermalink != "false" && isDIYBackend {
				displayOpts.SuppressPermalink = true
			}

			if err := validatePolicyPackConfig(args.PolicyPackPaths, args.PolicyPackConfigPaths); err != nil {
				return result.FromError(err)
			}

			s, err := requireStack(ctx, args.Stack, stackOfferNew, displayOpts)
			if err != nil {
				return result.FromError(err)
			}

			// Save any config values passed via flags.
			if err = parseAndSaveConfigArray(s, args.ConfigArray, args.ConfigPath); err != nil {
				return result.FromError(err)
			}

			proj, root, err := readProjectForUpdate(args.Client)
			if err != nil {
				return result.FromError(err)
			}

			m, err := getUpdateMetadata(
				args.Message,
				root,
				args.ExecKind,
				args.ExecAgent,
				args.PlanFilePath != "",
				cmd.Flags(),
			)
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
				ctx,
				stackName,
				proj,
				cfg.Environment,
				cfg.Config,
				encrypter,
				decrypter)
			if configErr != nil {
				return result.FromError(fmt.Errorf("validating stack config: %w", configErr))
			}

			targetURNs := []string{}
			targetURNs = append(targetURNs, args.Targets...)

			replaceURNs := []string{}
			replaceURNs = append(replaceURNs, args.Replaces...)

			for _, tr := range args.TargetReplaces {
				targetURNs = append(targetURNs, tr)
				replaceURNs = append(replaceURNs, tr)
			}

			refreshOption, err := getRefreshOption(proj, args.Refresh)
			if err != nil {
				return result.FromError(err)
			}

			opts := backend.UpdateOptions{
				Engine: engine.UpdateOptions{
					LocalPolicyPacks:          engine.MakeLocalPolicyPacks(args.PolicyPackPaths, args.PolicyPackConfigPaths),
					Parallel:                  args.Parallel,
					Debug:                     args.Debug,
					Refresh:                   refreshOption,
					ReplaceTargets:            deploy.NewUrnTargets(replaceURNs),
					UseLegacyDiff:             useLegacyDiff(),
					UseLegacyRefreshDiff:      useLegacyRefreshDiff(),
					DisableProviderPreview:    disableProviderPreview(),
					DisableResourceReferences: disableResourceReferences(),
					DisableOutputValues:       disableOutputValues(),
					Targets:                   deploy.NewUrnTargets(targetURNs),
					TargetDependents:          args.TargetDependents,
					// If we're trying to save a plan then we _need_ to generate it. We also turn this on in
					// experimental mode to just get more testing of it.
					GeneratePlan: hasExperimentalCommands() || args.PlanFilePath != "",
					Experimental: hasExperimentalCommands(),
				},
				Display: displayOpts,
			}

			// If we're building an import file we want to hook the event stream from the engine to transform
			// create operations into import specs.
			var importFilePromise *promise.Promise[importFile]
			var events chan engine.Event
			if args.ImportFilePath != "" {
				events = make(chan engine.Event)
				importFilePromise = buildImportFile(events)
			}

			plan, changes, res := s.Preview(ctx, backend.UpdateOperation{
				Proj:               proj,
				Root:               root,
				M:                  m,
				Opts:               opts,
				StackConfiguration: cfg,
				SecretsManager:     sm,
				SecretsProvider:    stack.DefaultSecretsProvider,
				Scopes:             backend.CancellationScopes,
			}, events)
			// If we made an events channel then we need to close it to trigger the exit of the import goroutine above.
			// The engine doesn't close the channel for us, but once its returned here we know it won't append any more
			// events.
			if events != nil {
				close(events)
			}

			switch {
			case res != nil:
				return PrintEngineResult(res)
			case args.ExpectNoChanges && changes != nil && engine.HasChanges(changes):
				return result.FromError(errors.New("error: no changes were expected but changes were proposed"))
			default:
				if args.PlanFilePath != "" {
					encrypter, err := sm.Encrypter()
					if err != nil {
						return result.FromError(err)
					}
					if err = writePlan(args.PlanFilePath, plan, encrypter, args.ShowSecrets); err != nil {
						return result.FromError(err)
					}

					// Write out message on how to use the plan (if not writing out --json)
					if !args.JSON {
						var buf bytes.Buffer
						fprintf(&buf, "Update plan written to '%s'", args.PlanFilePath)
						fprintf(
							&buf,
							"\nRun `pulumi up --plan='%s'` to constrain the update to the operations planned by this preview",
							args.PlanFilePath)
						cmdutil.Diag().Infof(diag.RawMessage("" /*urn*/, buf.String()))
					}
				}
				if importFilePromise != nil {
					importFile, err := importFilePromise.Result(ctx)
					if err != nil {
						return result.FromError(err)
					}

					f, err := os.Create(args.ImportFilePath)
					if err != nil {
						return result.FromError(err)
					}
					err = writeImportFile(importFile, f)
					err = errors.Join(err, f.Close())
					if err != nil {
						return result.FromError(err)
					}
				}
				return nil
			}
		}),
	}

	parentPulumiCmd.AddCommand(cmd)
	BindFlags[PreviewArgs](v, cmd)

	// TODO: hack/pulumirc -- support these?
	cmd.PersistentFlags().Lookup("refresh").NoOptDefVal = "true"
	cmd.Flag("suppress-permalink").NoOptDefVal = "false"
	_ = cmd.PersistentFlags().MarkHidden("client")
	if !hasDebugCommands() {
		_ = cmd.PersistentFlags().MarkHidden("event-log")
	}
	if !hasExperimentalCommands() {
		_ = cmd.PersistentFlags().MarkHidden("save-plan")
	}

	// TODO: hack/pulumirc stackConfigFile filth

	// Remote flags
	remoteArgs.applyFlags(cmd)

	// ignore err, only happens if flag does not exist
	_ = cmd.PersistentFlags().MarkHidden("exec-kind")
	// ignore err, only happens if flag does not exist
	_ = cmd.PersistentFlags().MarkHidden("exec-agent")

	return cmd
}

// buildImportFile takes an event stream from the engine and builds an import file from it for every create.
func buildImportFile(events <-chan engine.Event) *promise.Promise[importFile] {
	return promise.Run(func() (importFile, error) {
		// We may exit the below loop early if we encounter an error, so we need to make sure we drain the events
		// channel.
		defer func() {
			for range events {
			}
		}()

		// A mapping of every URN we see to it's name, used to build later resourceSpecs
		fullNameTable := map[resource.URN]string{}
		// A set of all URNs we've added to the import list, used to avoid adding parents to NameTable.
		importSet := map[resource.URN]struct{}{}
		// All providers that we've seen so far, used to build Version and PluginDownloadURL.
		providerInputs := map[resource.URN]resource.PropertyMap{}

		imports := importFile{
			NameTable: map[string]resource.URN{},
		}

		// A mapping of names to the index of the resourceSpec in imports.Resources that used it. We have to
		// fix up names _as we go_ because we're mapping over the event stream, and it would be pretty
		// inefficient to wait for the whole thing to finish before building the import specs.
		takenNames := map[string]int{}

		// We want to prefer using the urns name for the source name, but if it conflicts with other resources
		// we'll auto suffix it, first with the type, then with rising numbers. This function does that
		// auto-suffixing.
		uniqueName := func(name string, typ tokens.Type) string {
			caser := cases.Title(language.English, cases.NoLower)
			typeSuffix := caser.String(string(typ.Name()))
			baseName := fmt.Sprintf("%s%s", name, typeSuffix)
			name = baseName

			counter := 2
			for _, has := takenNames[name]; has; _, has = takenNames[name] {
				name = fmt.Sprintf("%s%d", baseName, counter)
				counter++
			}
			return name
		}

		// This is a pretty trivial mapping of Create operations to import declarations.
		for e := range events {
			preEvent, ok := e.Payload().(engine.ResourcePreEventPayload)
			if !ok {
				continue
			}

			urn := preEvent.Metadata.URN
			name := urn.Name()
			if i, has := takenNames[name]; has {
				// Another resource already has this name, lets check if that was it's original name or if it was a rename
				importI := imports.Resources[i]
				if importI.LogicalName != "" {
					// i was renamed, so we're going to go backwards rename it again and then we can use our name for this resource.
					newName := uniqueName(importI.LogicalName, importI.Type)
					imports.Resources[i].Name = newName
					// Go through all the resources and fix up any parent references to use the new name.
					for j := range imports.Resources {
						if imports.Resources[j].Parent == name {
							imports.Resources[j].Parent = newName
						}
					}
					// Fix up the nametable if needed
					if urn, has := imports.NameTable[name]; has {
						delete(imports.NameTable, name)
						imports.NameTable[newName] = urn
					}
					// Fix up takenNames incase this is hit again
					takenNames[newName] = i
				} else {
					// i just had the same name as us, lets find a new one
					name = uniqueName(name, urn.Type())
				}
			}

			// Name is unique at this point
			fullNameTable[urn] = name

			// If this is a provider we need to note we've seen it so we can build the Version and PluginDownloadURL of
			// any resources that use it.
			if providers.IsProviderType(urn.Type()) {
				providerInputs[urn] = preEvent.Metadata.Res.Inputs
			}

			// Only interested in creates
			if preEvent.Metadata.Op != deploy.OpCreate {
				continue
			}
			// No need to import the root stack even if it needs creating
			if preEvent.Metadata.Type == resource.RootStackType {
				continue
			}

			// We're importing this URN so track that we've seen it.
			importSet[urn] = struct{}{}

			// We can't actually import providers yet, just skip them. We'll only error if anything
			// actually tries to use it.
			if providers.IsProviderType(urn.Type()) {
				continue
			}

			new := preEvent.Metadata.New
			contract.Assertf(new != nil, "%s: expected new resource for a create to be non-nil", urn)

			var parent string
			if new.Parent != "" {
				// If the parent is just the root stack then skip it as we don't need to import that.
				if new.Parent.QualifiedType() != resource.RootStackType {
					var has bool
					parent, has = fullNameTable[new.Parent]
					contract.Assertf(has, "expected parent %q to be in full name table", new.Parent)
					// Don't add to the import NameTable if we're importing this in the same deployment.
					if _, has := importSet[new.Parent]; !has {
						imports.NameTable[parent] = new.Parent
					}
				}
			}

			var provider, version, pluginDownloadURL string
			if new.Provider != "" {
				ref, err := providers.ParseReference(new.Provider)
				if err != nil {
					return importFile{}, fmt.Errorf("could not parse provider reference: %w", err)
				}

				// If we're trying to create this provider in the same deployment and it's not a default provider then
				// we need to error, the import system can't yet "import" providers.
				if !providers.IsDefaultProvider(ref.URN()) {
					if _, has := importSet[ref.URN()]; has {
						return importFile{}, fmt.Errorf("cannot import resource %q with a new explicit provider %q", new.URN, ref.URN())
					}

					var has bool
					provider, has = fullNameTable[ref.URN()]
					contract.Assertf(has, "expected provider %q to be in full name table", new.Provider)

					imports.NameTable[provider] = ref.URN()
				}

				inputs, has := providerInputs[ref.URN()]
				contract.Assertf(has, "expected provider %q to be in provider inputs table", ref)

				v, err := providers.GetProviderVersion(inputs)
				if err != nil {
					return importFile{}, fmt.Errorf("could not get provider version for %s: %w", ref, err)
				}
				if v != nil {
					version = v.String()
				}

				pluginDownloadURL, err = providers.GetProviderDownloadURL(inputs)
				if err != nil {
					return importFile{}, fmt.Errorf("could not get provider download url for %s: %w", ref, err)
				}
			}

			var id resource.ID
			// id only needs filling in for custom resources, set it to a placeholder so the user can easily
			// search for that.
			if new.Custom {
				id = "<PLACEHOLDER>"
			}

			// We only want to set logical name if we need to
			var logicalName string
			if name != urn.Name() {
				logicalName = urn.Name()
			}

			takenNames[name] = len(imports.Resources)
			imports.Resources = append(imports.Resources, importSpec{
				Type:              new.Type,
				Name:              name,
				ID:                id,
				Parent:            parent,
				Provider:          provider,
				Component:         !new.Custom,
				Remote:            !new.Custom && new.Provider != "",
				Version:           version,
				PluginDownloadURL: pluginDownloadURL,
				LogicalName:       logicalName,
			})
		}

		return imports, nil
	})
}
