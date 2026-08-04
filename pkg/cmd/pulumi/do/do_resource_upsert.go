// Copyright 2026, Pulumi Corporation.
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

package do

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/blang/semver"
	"github.com/gofrs/uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	backendSecrets "github.com/pulumi/pulumi/pkg/v3/backend/secrets"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/autonames"
	cmdConfig "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/config"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/metadata"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	deployproviders "github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	sdkproviders "github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	codegenrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/codegen"
)

// StatefulUpdateRequest describes a single-snippet stateful update. The `runStatefulUpdate` hook
// converts this into a backend.UpdateOperation targeting only the named snippet, so existing state
// in the stack is untouched.
//
// The stack is opened and its snapshot inspected by the CLI layer (so it can resolve the snippet's
// UUID against any existing snippet of the same Name+Type); the resolved Stack is passed through
// here so the hook doesn't repeat the load.
type StatefulUpdateRequest struct {
	Snippets    []resource.Snippet
	Stack       backend.Stack
	DryRun      bool
	Yes         bool
	ShowSecrets bool
	Delete      bool
	Proj        *workspace.Project
	Root        string
	Sink        diag.Sink
}

// StatefulUpdateResult carries whatever the caller wants to render after the update.
type StatefulUpdateResult struct {
	// SnippetUUIDs in the same order as StatefulUpdateRequest.Snippets.
	SnippetUUIDs []string
}

// RunStatefulUpdateFunc is the injection point for driving the backend update/preview operation.
// NewDoCmd assigns the default implementation (real backend + engine); tests substitute a capturing
// stub so the CLI-level construction of the snippet and target can be exercised without a live
// backend.
//
// The stack is passed in via req rather than looked up here — the CLI layer needs the snapshot
// first (to resolve the snippet UUID) so it holds the stack open and hands it through.
type RunStatefulUpdateFunc func(
	ctx context.Context, flags *pflag.FlagSet, req StatefulUpdateRequest,
) (*StatefulUpdateResult, error)

func (pc *packageCommand) newStatefulResourceUpsertCommand(res *schema.Resource) *cobra.Command {
	var inputFile string
	var inputFormat string
	var resourcesFile string
	var yes bool
	cmd := &cobra.Command{
		Use:   "upsert <name>",
		Short: "Create a resource or fully update an existing one",
		Long: "Create a resource or fully update an existing one.\n\n" +
			"The resource created or updated is tracked in the stack, " +
			"so Pulumi can manage its lifecycle. No other resources in " +
			"the stack are affected when running this command.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			contract.Assertf(!pc.stateless, "upsert should not be registered in stateless mode")
			return pc.runStatefulSnippetUpdate(cmd, statefulSnippetUpdate{
				res:           res,
				name:          args[0],
				inputFile:     inputFile,
				inputFormat:   inputFormat,
				resourcesFile: resourcesFile,
				yes:           yes,
				requireFresh:  false,
			})
		},
	}
	addStatefulSnippetUpdateFlags(cmd, &inputFile, &inputFormat, &resourcesFile, &yes, res.InputProperties)
	return cmd
}

func (pc *packageCommand) newStatelessResourceUpsertCommand(res *schema.Resource) *cobra.Command {
	var inputFile string
	var inputFormat string
	var yes bool
	cmd := &cobra.Command{
		Use:   "upsert <id>",
		Short: "Create a resource or fully update an existing one",
		Long: "Create a resource or fully update an existing one.\n\n" +
			"Reads the resource with the given ID: if it exists, its inputs are fully " +
			"replaced with the given inputs (unlike `patch`, which merges them into the " +
			"existing inputs); otherwise a new resource is created, with an ID assigned " +
			"by the provider.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			contract.Assertf(pc.stateless, "stateless upsert should not be registered in stateful mode")
			if err := pc.requireYesIfNonInteractive(yes); err != nil {
				return err
			}
			ctx := cmd.Context()
			if err := pc.configureProvider(cmd, ctx); err != nil {
				return err
			}
			urn := resourceURN(res)
			id := resource.ID(args[0])
			read, err := pc.provider.Read(ctx, plugin.ReadRequest{
				URN:    urn,
				Name:   urn.Name(),
				Type:   urn.Type(),
				ID:     id,
				Inputs: resource.PropertyMap{},
				State:  resource.PropertyMap{},
			})
			if err != nil {
				return err
			}
			inputs, err := evaluateResourceFile(
				ctx, inputFile, "input", inputFormat, res, pc.evalContext(),
				pc.converter, pc.loaderTarget, pc.packageDescriptor,
				collectInputFlags(cmd, "input", res.InputProperties),
			)
			if err != nil {
				return fmt.Errorf("parse input file: %w", err)
			}
			if readNotFound(read) {
				return pc.runStatelessCreate(cmd, res, yes, func() (resource.PropertyMap, error) {
					return inputs, nil
				})
			}
			if read.ID != "" {
				id = read.ID
			}
			return pc.runStatelessUpdate(cmd, res, id, read, inputs, "update", yes)
		},
	}
	cmd.Flags().StringVar(&inputFormat, "input", "yaml", "Format of the resource inputs file")
	cmd.Flags().StringVar(&inputFile, "input-file", "", "Path to a file containing resource inputs")
	cmd.Flags().BoolVar(&yes, "yes", false,
		"Automatically approve and perform the operation without a confirmation prompt")
	addInputFlags(cmd, "input", res.InputProperties)
	return cmd
}

// statefulSnippetUpdate carries the pieces of a stateful snippet-add operation (create / upsert)
// that vary between commands. Everything else — parsing the input file, loading the stack,
// resolving the snippet UUID, and dispatching to runStatefulUpdate — is shared.
type statefulSnippetUpdate struct {
	res           *schema.Resource
	name          string
	inputFile     string
	inputFormat   string
	resourcesFile string
	yes           bool
	// requireFresh errors when a snippet with the same (Name, Type) already exists in the stack —
	// the invariant `create` enforces to distinguish itself from `upsert`.
	requireFresh bool
}

// runStatefulSnippetUpdate is the shared body of `create` (with requireFresh=true) and `upsert`
// (with requireFresh=false). Both take the same inputs, differ only in the pre-run policy check
// against any existing snippet with the same (Name, Type).
func (pc *packageCommand) runStatefulSnippetUpdate(cmd *cobra.Command, args statefulSnippetUpdate) error {
	contract.Assertf(pc.runStatefulUpdate != nil, "stateful snippet update is not wired up in this build")

	if pc.proj == nil {
		return fmt.Errorf("`%s` requires a Pulumi project (run inside a project directory)", cmd.Name())
	}
	if err := pc.requireYesIfNonInteractive(args.yes); err != nil {
		return err
	}

	ctx := cmd.Context()
	// Open the stack up front so we can look at the existing snapshot before deciding whether
	// this operation is legal (create requires a fresh snippet, upsert accepts either). The
	// stack is threaded through to runStatefulUpdate so it doesn't re-load. We also use the same
	// snapshot to resolve resource-reference package metadata before conversion.
	displayOpts := display.Options{Color: cmdutil.GetGlobalColorization()}
	stack, err := cmdStack.RequireStack(
		ctx, pc.diagFwd, pc.ws, pc.lm,
		"",                                 /*stackName — use currently selected*/
		cmdStack.LoadOnly, displayOpts, "", /*configFile*/
	)
	if err != nil {
		return fmt.Errorf("load stack: %w", err)
	}
	snap, err := stack.Snapshot(ctx, backendSecrets.DefaultProvider)
	if err != nil {
		return fmt.Errorf("load stack snapshot: %w", err)
	}

	userResources, err := readResourceReferences(args.resourcesFile)
	if err != nil {
		return err
	}
	// Merge the auto-assigned identifiers (derived from the stack snapshot) with the user's
	// --resources-file. User entries win on collision.
	resources := autonames.Merge(autonames.ResourceNames(snap), userResources)
	resourceInfos, err := resourceReferenceInfos(resources, snap)
	if err != nil {
		return err
	}

	// Merge --input-* flags into the file's PCL AST so the persisted snippet body matches what
	// the user typed on the command line. If no file was provided, the flags become the snippet
	// body by themselves.
	inputFlags := collectInputFlags(cmd, "input", args.res.InputProperties)
	code, resourceFilename, resourceNames, err := parseFile(
		ctx, args.inputFile, "input", args.inputFormat, args.res.Token,
		pc.converter, pc.loaderTarget, pc.packageDescriptor, inputFlags, resourceInfos,
	)
	if err != nil {
		return fmt.Errorf("read input file: %w", err)
	}
	references, err := applyResourceNameRemaps(resources, resourceNames)
	if err != nil {
		return err
	}

	// Snippet identity in the snapshot is (Name, Type) — reuse the existing UUID so the engine's
	// applySnippetUpdates path replaces the snippet in place rather than adding a duplicate that
	// would then race to register the same URN.
	snippetUUID, existed, err := resolveSnippetUUID(snap, args.name, args.res.Token)
	if err != nil {
		return err
	}
	if args.requireFresh && existed {
		return fmt.Errorf("resource %s %q already exists in stack %s; use `upsert` to replace it",
			args.res.Token, args.name, stack.Ref())
	}
	// Pick an identifier for the injected provider reference that doesn't collide with a
	// user-supplied resource reference of the same name.
	providerRefName := "provider"
	for i := 2; ; i++ {
		if _, taken := references[providerRefName]; !taken {
			break
		}
		providerRefName = fmt.Sprintf("provider%d", i)
	}
	providerSnippet, provReferences, resourceCode, err := pc.buildProviderSnippet(
		ctx, cmd, snap, stack, args.name+"-provider", providerRefName, code, resourceFilename,
	)
	if err != nil {
		return err
	}

	mergedReferences := references
	for k, v := range provReferences {
		if mergedReferences == nil {
			mergedReferences = map[string]string{}
		}
		mergedReferences[k] = v
	}
	// Trim the reference map to just the identifiers the resource's PCL body actually references
	// before persisting. Auto-derived entries that aren't used would freeze URNs from the current
	// snapshot into state and go stale if those resources change. The provider-injected identifier
	// (in provReferences) is always kept — buildProviderSnippet injected it into resourceCode.
	if used := referencedIdentsInPCL(resourceCode, resourceFilename); used != nil {
		mergedReferences = filterReferencesByUsage(mergedReferences, used)
	}

	snippet := resource.Snippet{
		UUID:       snippetUUID,
		Name:       args.name,
		Type:       args.res.Token,
		Code:       string(resourceCode),
		Descriptor: packageDescriptorFromProto(pc.packageDescriptor),
		References: mergedReferences,
	}

	snippets := []resource.Snippet{}
	if providerSnippet != nil {
		snippets = append(snippets, *providerSnippet)
	}
	snippets = append(snippets, snippet)

	result, err := pc.runStatefulUpdate(ctx, cmd.Flags(), StatefulUpdateRequest{
		Snippets:    snippets,
		Stack:       stack,
		DryRun:      pc.dryrun,
		Yes:         args.yes,
		ShowSecrets: pc.showSecrets,
		Proj:        pc.proj,
		Root:        pc.root,
		Sink:        pc.diagFwd,
	})
	if err != nil {
		return err
	}
	if result != nil && !pc.dryrun && len(result.SnippetUUIDs) > 0 {
		verb := "Created"
		if existed {
			verb = "Updated"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s %s (snippet %s)\n",
			verb, args.name, result.SnippetUUIDs[len(result.SnippetUUIDs)-1])
	}
	return nil
}

func (pc *packageCommand) runStatefulSnippetDelete(
	cmd *cobra.Command, res *schema.Resource, name string, yes bool,
) error {
	contract.Assertf(pc.runStatefulUpdate != nil, "stateful snippet update is not wired up in this build")

	if pc.proj == nil {
		return fmt.Errorf("`%s` requires a Pulumi project (run inside a project directory)", cmd.Name())
	}
	if err := pc.requireYesIfNonInteractive(yes); err != nil {
		return err
	}

	ctx := cmd.Context()
	displayOpts := display.Options{Color: cmdutil.GetGlobalColorization()}
	stack, err := cmdStack.RequireStack(
		ctx, pc.diagFwd, pc.ws, pc.lm,
		"",                                 /*stackName — use currently selected*/
		cmdStack.LoadOnly, displayOpts, "", /*configFile*/
	)
	if err != nil {
		return fmt.Errorf("load stack: %w", err)
	}
	snap, err := stack.Snapshot(ctx, backendSecrets.DefaultProvider)
	if err != nil {
		return fmt.Errorf("load stack snapshot: %w", err)
	}

	snippetUUID, exists, err := resolveSnippetUUID(snap, name, res.Token)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("resource %s %q does not exist in stack %s", res.Token, name, stack.Ref())
	}

	result, err := pc.runStatefulUpdate(ctx, cmd.Flags(), StatefulUpdateRequest{
		Snippets: []resource.Snippet{{
			UUID: snippetUUID,
			Name: name,
			Type: res.Token,
		}},
		Stack:       stack,
		DryRun:      pc.dryrun,
		Yes:         yes,
		ShowSecrets: pc.showSecrets,
		Delete:      true,
		Proj:        pc.proj,
		Root:        pc.root,
		Sink:        pc.diagFwd,
	})
	if err != nil {
		return err
	}
	if result != nil && !pc.dryrun && len(result.SnippetUUIDs) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Deleted %s (snippet %s)\n", name, result.SnippetUUIDs[0])
	}
	return nil
}

// addStatefulSnippetUpdateFlags installs the flag set shared by stateful `create` and `upsert`.
func addStatefulSnippetUpdateFlags(
	cmd *cobra.Command, inputFile, inputFormat, resourcesFile *string, yes *bool, inputs []*schema.Property,
) {
	cmd.Flags().StringVar(inputFile, "input-file", "", "Path to a file containing resource inputs")
	cmd.Flags().StringVar(inputFormat, "input", "yaml",
		"Format of the resource inputs file (any language name supported by an installed converter)")
	cmd.Flags().StringVar(resourcesFile, "resources-file", "",
		"Path to a JSON file mapping identifiers to resource URNs that input expressions may reference.\n"+
			"The file must be a JSON object whose keys are the identifiers used in input expressions and\n"+
			"whose values are the URNs of existing resources in the stack, for example:\n"+
			"  {\n"+
			"    \"myBucket\": \"urn:pulumi:dev::my-project::aws:s3/bucket:Bucket::my-bucket\",\n"+
			"    \"myVpc\":    \"urn:pulumi:dev::my-project::aws:ec2/vpc:Vpc::my-vpc\"\n"+
			"  }\n"+
			"Identifiers for existing stack resources are auto-assigned; run `pulumi do --resources`\n"+
			"to see them. Entries in this file take precedence over any auto-assigned identifier.")
	cmd.Flags().BoolVar(yes, "yes", false,
		"Automatically approve and perform the operation without a confirmation prompt")
	addInputFlags(cmd, "input", inputs)
}

func readResourceReferences(path string) (map[string]string, error) {
	if path == "" {
		return nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open resources file: %w", err)
	}
	defer contract.IgnoreClose(f)

	var refs map[string]string
	if err := json.NewDecoder(f).Decode(&refs); err != nil {
		return nil, fmt.Errorf("parse resources file: %w", err)
	}
	for name, rawURN := range refs {
		if name == "" {
			return nil, errors.New("resources file contains an empty resource name")
		}
		urn, err := resource.ParseURN(rawURN)
		if err != nil {
			return nil, fmt.Errorf("resources file contains invalid URN for %q: %w", name, err)
		}
		refs[name] = string(urn)
	}
	return refs, nil
}

func resourceReferenceInfos(
	resources map[string]string, snap *deploy.Snapshot,
) (map[string]plugin.ConvertSnippetResourceReference, error) {
	if len(resources) == 0 {
		return nil, nil
	}

	statesByURN := map[resource.URN]*pkgresource.State{}
	providersByRef := map[string]*pkgresource.State{}
	if snap != nil {
		for _, state := range snap.Resources {
			if state == nil {
				continue
			}
			statesByURN[state.URN] = state
			if sdkproviders.IsProviderType(state.Type) {
				ref, err := sdkproviders.NewReference(state.URN, state.ID)
				if err != nil {
					return nil, fmt.Errorf("could not build provider reference for %s: %w", state.URN, err)
				}
				providersByRef[ref.String()] = state
			}
		}
	}

	refs := make(map[string]plugin.ConvertSnippetResourceReference, len(resources))
	for name, rawURN := range resources {
		urn := resource.URN(rawURN)
		state, ok := statesByURN[urn]
		if !ok {
			return nil, fmt.Errorf("resources file references %q as %s, but that resource was not found in the stack", name, urn)
		}

		typ := state.Type
		pkg := typ.Package()
		if sdkproviders.IsProviderType(typ) {
			pkg = sdkproviders.GetProviderPackage(typ)
		}
		packageReq := &codegenrpc.GetSchemaRequest{
			Package: string(pkg),
		}

		providerState, err := resourceReferenceProviderState(state, providersByRef)
		if err != nil {
			return nil, fmt.Errorf("resources file reference %q: %w", name, err)
		}
		if providerState != nil {
			providerPackage := sdkproviders.GetProviderPackage(providerState.Type)
			schemaPackage, err := deployproviders.GetProviderName(providerPackage, providerState.Inputs)
			if err != nil {
				return nil, fmt.Errorf("resources file reference %q: get provider name: %w", name, err)
			}
			packageReq.Package = string(schemaPackage)

			version, err := deployproviders.GetProviderVersion(providerState.Inputs)
			if err != nil {
				return nil, fmt.Errorf("resources file reference %q: get provider version: %w", name, err)
			}
			if version != nil {
				packageReq.Version = version.String()
			}

			downloadURL, err := deployproviders.GetProviderDownloadURL(providerState.Inputs)
			if err != nil {
				return nil, fmt.Errorf("resources file reference %q: get provider download URL: %w", name, err)
			}
			packageReq.DownloadUrl = downloadURL

			parameterization, err := deployproviders.GetProviderParameterization(providerPackage, providerState.Inputs)
			if err != nil {
				return nil, fmt.Errorf("resources file reference %q: get provider parameterization: %w", name, err)
			}
			if parameterization != nil {
				packageReq.Parameterization = &codegenrpc.Parameterization{
					Name:    parameterization.Name,
					Version: parameterization.Version.String(),
					Value:   parameterization.Value,
				}
			}
		}

		refs[name] = plugin.ConvertSnippetResourceReference{
			Token:   string(typ),
			Package: packageReq,
		}
	}
	return refs, nil
}

func resourceReferenceProviderState(
	state *pkgresource.State, providersByRef map[string]*pkgresource.State,
) (*pkgresource.State, error) {
	if sdkproviders.IsProviderType(state.Type) {
		return state, nil
	}
	if state.Provider == "" {
		return nil, nil
	}
	providerState, ok := providersByRef[state.Provider]
	if !ok {
		return nil, fmt.Errorf("provider %s was not found in the stack", state.Provider)
	}
	return providerState, nil
}

func applyResourceNameRemaps(resources, resourceNames map[string]string) (map[string]string, error) {
	if len(resources) == 0 {
		return nil, nil
	}

	refs := make(map[string]string, len(resources))
	for oldName, urn := range resources {
		newName := oldName
		if renamed, ok := resourceNames[oldName]; ok {
			if renamed == "" {
				return nil, fmt.Errorf("converter returned an empty resource name for %q", oldName)
			}
			newName = renamed
		}
		if _, exists := refs[newName]; exists {
			return nil, fmt.Errorf("converter mapped multiple resources to %q", newName)
		}
		refs[newName] = urn
	}

	for oldName, newName := range resourceNames {
		if _, ok := resources[oldName]; !ok {
			return nil, fmt.Errorf("converter returned a resource name mapping for unknown resource %q", oldName)
		}
		if newName == "" {
			return nil, fmt.Errorf("converter returned an empty resource name for %q", oldName)
		}
	}

	return refs, nil
}

// resolveSnippetUUID looks up an existing snippet in snap matching (name, resourceToken) and
// returns its UUID for reuse (with existed=true); otherwise it mints a fresh UUIDv4 (existed=false).
// Callers use existed to enforce operation-specific invariants: stateful `create` errors when a
// snippet already exists, `upsert` doesn't care, and stateful `delete` errors when it doesn't.
//
// Snippet identity within a snapshot is (Name, Type): a second snippet with the same pair would
// register the same resource URN and race with the first, so any resolver that reuses an existing
// entry is preserving that invariant.
func resolveSnippetUUID(snap *deploy.Snapshot, name, resourceToken string) (string, bool, error) {
	if snap != nil {
		for _, existing := range snap.Snippets {
			if existing.Name == name && existing.Type == resourceToken {
				return existing.UUID, true, nil
			}
		}
	}
	fresh, err := uuid.NewV4()
	if err != nil {
		return "", false, fmt.Errorf("generate snippet uuid: %w", err)
	}
	return fresh.String(), false, nil
}

// buildProviderSnippet decides whether the resource being upserted needs an inline provider
// snippet. It returns (providerSnippet, resourceReferences, updatedResourceCode) for the caller to
// stitch into the resource snippet. Three cases:
//   - Default provider: no --provider, no provider overrides — returns (nil, nil, resourceCode).
//   - Bare --provider: --provider set, no overrides — returns (nil, references, resourceCode with
//     options { provider = provider }).
//   - Materialize: provider overrides given (with or without --provider) — returns a provider
//     snippet whose Code carries the overrides overlaid on top of any base --provider inputs, plus
//     the resource references + injected options block.
func (pc *packageCommand) buildProviderSnippet(
	ctx context.Context, cmd *cobra.Command, snap *deploy.Snapshot,
	stack backend.Stack, providerName, providerRefName string, resourceCode []byte, resourceFilename string,
) (*resource.Snippet, map[string]string, []byte, error) {
	providerFlags := collectInputFlags(cmd, pc.spec.Name(), pc.providerDef.InputProperties)
	hasOverrides := pc.providerFile != "" || len(providerFlags) > 0
	if pc.providerURN == "" && !hasOverrides {
		return nil, nil, resourceCode, nil
	}

	stackShortName := stack.Ref().Name().String()
	providerType := "pulumi:providers:" + pc.spec.Name()

	providerURN := resource.URN(pc.providerURN)
	var providerSnippet *resource.Snippet
	if hasOverrides {
		providerCode, providerFilename, _, err := parseFile(
			ctx, pc.providerFile, "provider", pc.format, "",
			pc.converter, pc.loaderTarget, pc.packageDescriptor, providerFlags, nil,
		)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("parse provider file: %w", err)
		}

		if pc.providerURN != "" {
			base, err := pc.loadProviderInputsFromStack(ctx, resource.URN(pc.providerURN))
			if err != nil {
				return nil, nil, nil, fmt.Errorf("--provider: %w", err)
			}
			baseLiterals := make(map[string]string, len(base))
			for k, v := range base {
				// Skip engine bookkeeping (__internal) and the pinned plugin version — carrying
				// these into a new snippet would collide with the plugin selection the descriptor
				// already encodes.
				if k == "__internal" || k == "version" {
					continue
				}
				lit, err := propertyValueToPCLLiteral(string(k), v)
				if err != nil {
					return nil, nil, nil, fmt.Errorf("--provider: %w", err)
				}
				baseLiterals[string(k)] = lit
			}
			providerCode, err = mergeAbsentAttributeLiteralsIntoPCL(providerCode, providerFilename, "provider", baseLiterals)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("merge --provider base inputs: %w", err)
			}
		}

		providerURN = resource.CreateURN(providerName, providerType, "", string(pc.proj.Name), stackShortName)
		providerUUID, _, err := resolveSnippetUUID(snap, providerName, providerType)
		if err != nil {
			return nil, nil, nil, err
		}
		providerSnippet = &resource.Snippet{
			UUID:       providerUUID,
			Name:       providerName,
			Type:       providerType,
			Code:       string(providerCode),
			Descriptor: packageDescriptorFromProto(pc.packageDescriptor),
		}
	}

	refs := map[string]string{providerRefName: string(providerURN)}
	newCode, err := injectProviderOptionInPCL(resourceCode, resourceFilename, providerRefName)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("inject provider option: %w", err)
	}
	return providerSnippet, refs, newCode, nil
}

// DefaultRunStatefulUpdate is the production implementation of the runStatefulUpdate hook. The
// caller (typically the upsert command) has already loaded the stack and picked the snippet's
// UUID; this function loads config + secrets and calls the backend preview/update entrypoint with
// an UpdateOperation whose engine options carry the snippet and target it.
func DefaultRunStatefulUpdate(
	ctx context.Context, flags *pflag.FlagSet, req StatefulUpdateRequest,
) (*StatefulUpdateResult, error) {
	if req.Stack == nil {
		return nil, errors.New("stateful update requires a loaded stack")
	}
	displayOpts := display.Options{
		Color:         cmdutil.GetGlobalColorization(),
		ShowSecrets:   req.ShowSecrets,
		IsInteractive: cmdutil.Interactive(),
		ShowDiff:      true,
	}

	ssml := cmdStack.SecretsManagerLoader{FallbackToState: true}
	cfg, sm, err := cmdConfig.GetStackConfiguration(ctx, req.Sink, ssml, req.Stack, req.Proj, "", nil)
	if err != nil {
		return nil, fmt.Errorf("get stack configuration: %w", err)
	}

	m, err := metadata.GetUpdateMetadata("", req.Root, "", "", false, cfg, flags)
	if err != nil {
		return nil, fmt.Errorf("gathering environment metadata: %w", err)
	}
	cmdutil.SetStringSpanAttributes(ctx, m.Environment)

	if len(req.Snippets) == 0 {
		return nil, errors.New("stateful update requires at least one snippet")
	}
	snippets := map[uuid.UUID]*resource.Snippet{}
	targetSnippets := make([]string, 0, len(req.Snippets))
	uuids := make([]string, 0, len(req.Snippets))
	for i := range req.Snippets {
		s := req.Snippets[i]
		snippetUUIDVal, err := uuid.FromString(s.UUID)
		if err != nil {
			return nil, fmt.Errorf("snippet uuid: %w", err)
		}
		if _, dup := snippets[snippetUUIDVal]; dup {
			return nil, fmt.Errorf("duplicate snippet uuid %s in stateful update request", s.UUID)
		}
		if req.Delete {
			snippets[snippetUUIDVal] = nil
		} else {
			snippet := s
			snippets[snippetUUIDVal] = &snippet
		}
		targetSnippets = append(targetSnippets, snippetUUIDVal.String())
		uuids = append(uuids, s.UUID)
	}

	engineOpts := engine.UpdateOptions{
		Snippets:       snippets,
		TargetSnippets: targetSnippets,
		ShowSecrets:    req.ShowSecrets,
	}

	op := backend.UpdateOperation{
		Proj:               req.Proj,
		Root:               req.Root,
		M:                  m,
		Opts:               backend.UpdateOptions{Engine: engineOpts, Display: displayOpts, AutoApprove: req.Yes},
		StackConfiguration: cfg,
		SecretsManager:     sm,
		SecretsProvider:    backendSecrets.DefaultProvider,
		Scopes:             backend.CancellationScopes,
	}

	if req.DryRun {
		err = backend.RunCollectingDiff(op.Opts.Display, nil, func(events chan<- engine.Event) error {
			_, _, e := backend.PreviewStack(ctx, req.Stack, op, events)
			return e
		})
	} else {
		_, err = backend.UpdateStack(ctx, req.Stack, op, nil /* events */)
	}
	if err != nil {
		return nil, err
	}

	return &StatefulUpdateResult{SnippetUUIDs: uuids}, nil
}

// packageDescriptorFromProto lifts the codegen-RPC schema request into the resource-layer
// PackageDescriptor stored in snippets. Snippets are serialized into state, so the descriptor must
// carry enough information for a later run to reload the same (possibly parameterized) package.
func packageDescriptorFromProto(req *codegenrpc.GetSchemaRequest) resource.PackageDescriptor {
	out := resource.PackageDescriptor{Name: req.Package}
	if req.Version != "" {
		if v, err := semver.ParseTolerant(req.Version); err == nil {
			out.Version = &v
		}
	}
	if req.Parameterization != nil {
		desc := &resource.ParameterizationDescriptor{
			Name:  req.Parameterization.Name,
			Value: req.Parameterization.Value,
		}
		if v, err := semver.ParseTolerant(req.Parameterization.Version); err == nil {
			desc.Version = v
		}
		out.Parameterization = desc
	}
	return out
}
