// Copyright 2016-2024, Pulumi Corporation.
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

package operations

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/config"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/deployment"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/metadata"
	pkgPlan "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/plan"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/autonaming"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/promise"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

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

func NewPreviewCmd() *cobra.Command {
	var debug bool
	var expectNop bool
	var message string
	var execKind string
	var execAgent string
	var stackName string
	var configArray []string
	var configPath bool
	var client string
	var planFilePath string
	var importFilePath string
	var showSecrets bool

	// Flags for remote operations.
	remoteArgs := deployment.RemoteArgs{}

	// Flags for engine.UpdateOptions.
	var jsonDisplay bool
	var policyPackPaths []string
	var policyPackConfigPaths []string
	var diffDisplay bool
	var eventLogPath string
	var parallel int32
	var refresh string
	var showConfig bool
	var showPolicyRemediations bool
	var showReplacementSteps bool
	var showSames bool
	var showReads bool
	var suppressOutputs bool
	var suppressProgress bool
	var suppressPermalink string
	var targets []string
	var replaces []string
	var targetReplaces []string
	var targetDependents bool
	var attachDebugger bool

	use, cmdArgs := "preview", cmdutil.NoArgs
	if deployment.RemoteSupported() {
		use, cmdArgs = "preview [url]", cmdutil.MaximumNArgs(1)
	}

	cmd := &cobra.Command{
		Use:        use,
		Aliases:    []string{"pre"},
		SuggestFor: []string{"build", "plan"},
		Short:      "Show a preview of updates to a stack's resources",
		Long: "Show a preview of updates to a stack's resources.\n" +
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
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ssml := cmdStack.NewStackSecretsManagerLoaderFromEnv()
			ws := pkgWorkspace.Instance
			displayType := display.DisplayProgress
			if diffDisplay {
				displayType = display.DisplayDiff
			}

			displayOpts := display.Options{
				Color:                  cmdutil.GetGlobalColorization(),
				ShowConfig:             showConfig,
				ShowPolicyRemediations: showPolicyRemediations,
				ShowReplacementSteps:   showReplacementSteps,
				ShowSameResources:      showSames,
				ShowReads:              showReads,
				ShowSecrets:            showSecrets,
				SuppressOutputs:        suppressOutputs,
				SuppressProgress:       suppressProgress,
				IsInteractive:          cmdutil.Interactive(),
				Type:                   displayType,
				JSONDisplay:            jsonDisplay,
				EventLogPath:           eventLogPath,
				Debug:                  debug,
			}

			// we only suppress permalinks if the user passes true. the default is an empty string
			// which we pass as 'false'
			if suppressPermalink == "true" {
				displayOpts.SuppressPermalink = true
			} else {
				displayOpts.SuppressPermalink = false
			}

			if remoteArgs.Remote {
				err := deployment.ValidateUnsupportedRemoteFlags(expectNop, configArray, configPath, client, jsonDisplay,
					policyPackPaths, policyPackConfigPaths, refresh, showConfig, showPolicyRemediations,
					showReplacementSteps, showSames, showReads, suppressOutputs, "default", &targets, replaces,
					targetReplaces, targetDependents, planFilePath, cmdStack.ConfigFile)
				if err != nil {
					return err
				}

				var url string
				if len(args) > 0 {
					url = args[0]
				}

				if errResult := deployment.ValidateRemoteDeploymentFlags(url, remoteArgs); errResult != nil {
					return errResult
				}

				return deployment.RunDeployment(ctx, ws, cmd, displayOpts, apitype.Preview, stackName, url, remoteArgs)
			}

			isDIYBackend, err := cmdBackend.IsDIYBackend(ws, displayOpts)
			if err != nil {
				return err
			}

			// by default, we are going to suppress the permalink when using DIY backends
			// this can be re-enabled by explicitly passing "false" to the `suppress-permalink` flag
			if suppressPermalink != "false" && isDIYBackend {
				displayOpts.SuppressPermalink = true
			}

			if err := validatePolicyPackConfig(policyPackPaths, policyPackConfigPaths); err != nil {
				return err
			}

			s, err := cmdStack.RequireStack(
				ctx,
				ws,
				cmdBackend.DefaultLoginManager,
				stackName,
				cmdStack.OfferNew,
				displayOpts,
			)
			if err != nil {
				return err
			}

			// Save any config values passed via flags.
			if err = parseAndSaveConfigArray(ws, s, configArray, configPath); err != nil {
				return err
			}

			proj, root, err := readProjectForUpdate(ws, client)
			if err != nil {
				return err
			}

			cfg, sm, err := config.GetStackConfiguration(ctx, ssml, s, proj)
			if err != nil {
				return fmt.Errorf("getting stack configuration: %w", err)
			}

			m, err := metadata.GetUpdateMetadata(message, root, execKind, execAgent, planFilePath != "", cfg, cmd.Flags())
			if err != nil {
				return fmt.Errorf("gathering environment metadata: %w", err)
			}

			decrypter := sm.Decrypter()
			encrypter := sm.Encrypter()

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
				return fmt.Errorf("validating stack config: %w", configErr)
			}

			targetURNs := []string{}
			targetURNs = append(targetURNs, targets...)

			replaceURNs := []string{}
			replaceURNs = append(replaceURNs, replaces...)

			for _, tr := range targetReplaces {
				targetURNs = append(targetURNs, tr)
				replaceURNs = append(replaceURNs, tr)
			}

			refreshOption, err := getRefreshOption(proj, refresh)
			if err != nil {
				return err
			}

			autonamer, err := autonaming.ParseAutonamingConfig(autonamingStackContext(proj, s), cfg.Config, decrypter)
			if err != nil {
				return fmt.Errorf("getting autonaming config: %w", err)
			}

			opts := backend.UpdateOptions{
				Engine: engine.UpdateOptions{
					ParallelDiff:              env.ParallelDiff.Value(),
					LocalPolicyPacks:          engine.MakeLocalPolicyPacks(policyPackPaths, policyPackConfigPaths),
					Parallel:                  parallel,
					Debug:                     debug,
					ShowSecrets:               showSecrets,
					Refresh:                   refreshOption,
					ReplaceTargets:            deploy.NewUrnTargets(replaceURNs),
					UseLegacyDiff:             env.EnableLegacyDiff.Value(),
					UseLegacyRefreshDiff:      env.EnableLegacyRefreshDiff.Value(),
					DisableProviderPreview:    env.DisableProviderPreview.Value(),
					DisableResourceReferences: env.DisableResourceReferences.Value(),
					DisableOutputValues:       env.DisableOutputValues.Value(),
					Targets:                   deploy.NewUrnTargets(targetURNs),
					TargetDependents:          targetDependents,
					// If we're trying to save a plan then we _need_ to generate it. We also turn this on in
					// experimental mode to just get more testing of it.
					GeneratePlan:   env.Experimental.Value() || planFilePath != "",
					Experimental:   env.Experimental.Value(),
					AttachDebugger: attachDebugger,
					Autonamer:      autonamer,
				},
				Display: displayOpts,
			}

			// If we're building an import file we want to hook the event stream from the engine to transform
			// create operations into import specs.
			var importFilePromise *promise.Promise[importFile]
			var events chan engine.Event
			if importFilePath != "" {
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
				return res
			case expectNop && changes != nil && engine.HasChanges(changes):
				return errors.New("error: no changes were expected but changes were proposed")
			default:
				if planFilePath != "" {
					encrypter := sm.Encrypter()
					if err = pkgPlan.Write(planFilePath, plan, encrypter, showSecrets); err != nil {
						return err
					}

					// Write out message on how to use the plan (if not writing out --json)
					if !jsonDisplay {
						var buf bytes.Buffer
						ui.Fprintf(&buf, "Update plan written to '%s'", planFilePath)
						ui.Fprintf(
							&buf,
							"\nRun `pulumi up --plan='%s'` to constrain the update to the operations planned by this preview",
							planFilePath)
						cmdutil.Diag().Infof(diag.RawMessage("" /*urn*/, buf.String()))
					}
				}
				if importFilePromise != nil {
					importFile, err := importFilePromise.Result(ctx)
					if err != nil {
						return err
					}

					f, err := os.Create(importFilePath)
					if err != nil {
						return err
					}
					err = writeImportFile(importFile, f)
					err = errors.Join(err, f.Close())
					if err != nil {
						return err
					}
				}
				return nil
			}
		},
	}

	cmd.PersistentFlags().BoolVarP(
		&debug, "debug", "d", false,
		"Print detailed debugging output during resource operations")
	cmd.PersistentFlags().BoolVar(
		&expectNop, "expect-no-changes", false,
		"Return an error if any changes are proposed by this preview")
	cmd.PersistentFlags().StringVarP(
		&stackName, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.PersistentFlags().StringVar(
		&cmdStack.ConfigFile, "config-file", "",
		"Use the configuration values in the specified file rather than detecting the file name")
	cmd.PersistentFlags().StringArrayVarP(
		&configArray, "config", "c", []string{},
		"Config to use during the preview and save to the stack config file")
	cmd.PersistentFlags().BoolVar(
		&configPath, "config-path", false,
		"Config keys contain a path to a property in a map or list to set")
	cmd.PersistentFlags().StringVar(
		&planFilePath, "save-plan", "",
		"[EXPERIMENTAL] Save the operations proposed by the preview to a plan file at the given path")
	cmd.Flags().BoolVarP(
		&showSecrets, "show-secrets", "", false,
		"Show secrets in plaintext in the CLI output,"+
			" if used with --save-plan the secrets will also be shown in the plan file. Defaults to `false`")

	if !env.Experimental.Value() {
		contract.AssertNoErrorf(cmd.PersistentFlags().MarkHidden("save-plan"),
			`Could not mark "save-plan" as hidden`)
	}
	cmd.PersistentFlags().StringVar(
		&importFilePath, "import-file", "",
		"Save any creates seen during the preview into an import file to use with 'pulumi import'")

	cmd.PersistentFlags().StringVar(
		&client, "client", "", "The address of an existing language runtime host to connect to")
	_ = cmd.PersistentFlags().MarkHidden("client")

	cmd.PersistentFlags().StringVarP(
		&message, "message", "m", "",
		"Optional message to associate with the preview operation")

	cmd.PersistentFlags().StringArrayVarP(
		&targets, "target", "t", []string{},
		"Specify a single resource URN to update. Other resources will not be updated."+
			" Multiple resources can be specified using --target urn1 --target urn2")
	cmd.PersistentFlags().StringArrayVar(
		&replaces, "replace", []string{},
		"Specify resources to replace. Multiple resources can be specified using --replace urn1 --replace urn2")
	cmd.PersistentFlags().StringArrayVar(
		&targetReplaces, "target-replace", []string{},
		"Specify a single resource URN to replace. Other resources will not be updated."+
			" Shorthand for --target urn --replace urn.")
	cmd.PersistentFlags().BoolVar(
		&targetDependents, "target-dependents", false,
		"Allows updating of dependent targets discovered but not specified in --target list")

	// Flags for engine.UpdateOptions.
	cmd.PersistentFlags().StringSliceVar(
		&policyPackPaths, "policy-pack", []string{},
		"Run one or more policy packs as part of this update")
	cmd.PersistentFlags().StringSliceVar(
		&policyPackConfigPaths, "policy-pack-config", []string{},
		`Path to JSON file containing the config for the policy pack of the corresponding "--policy-pack" flag`)
	cmd.PersistentFlags().BoolVar(
		&diffDisplay, "diff", false,
		"Display operation as a rich diff showing the overall change")
	cmd.Flags().BoolVarP(
		&jsonDisplay, "json", "j", false,
		"Serialize the preview diffs, operations, and overall output as JSON")
	cmd.PersistentFlags().Int32VarP(
		&parallel, "parallel", "p", defaultParallel(),
		"Allow P resource operations to run in parallel at once (1 for no parallelism).")
	cmd.PersistentFlags().StringVarP(
		&refresh, "refresh", "r", "",
		"Refresh the state of the stack's resources before this update")
	cmd.PersistentFlags().Lookup("refresh").NoOptDefVal = "true"
	cmd.PersistentFlags().BoolVar(
		&showConfig, "show-config", false,
		"Show configuration keys and variables")
	cmd.PersistentFlags().BoolVar(
		&showPolicyRemediations, "show-policy-remediations", false,
		"Show per-resource policy remediation details instead of a summary")
	cmd.PersistentFlags().BoolVar(
		&showReplacementSteps, "show-replacement-steps", false,
		"Show detailed resource replacement creates and deletes instead of a single step")

	cmd.PersistentFlags().BoolVar(
		&showSames, "show-sames", false,
		"Show resources that needn't be updated because they haven't changed, alongside those that do")
	cmd.PersistentFlags().BoolVar(
		&showReads, "show-reads", false,
		"Show resources that are being read in, alongside those being managed directly in the stack")
	cmd.PersistentFlags().BoolVar(
		&suppressOutputs, "suppress-outputs", false,
		"Suppress display of stack outputs (in case they contain sensitive values)")
	cmd.PersistentFlags().BoolVar(
		&suppressProgress, "suppress-progress", false,
		"Suppress display of periodic progress dots")
	cmd.PersistentFlags().StringVar(
		&suppressPermalink, "suppress-permalink", "",
		"Suppress display of the state permalink")
	cmd.Flag("suppress-permalink").NoOptDefVal = "false"
	cmd.PersistentFlags().BoolVar(
		&attachDebugger, "attach-debugger", false,
		"Enable the ability to attach a debugger to the program being executed")

	// Remote flags
	remoteArgs.ApplyFlags(cmd)

	if env.DebugCommands.Value() {
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
