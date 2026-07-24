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
	"errors"
	"fmt"

	"github.com/blang/semver"
	"github.com/gofrs/uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	backendSecrets "github.com/pulumi/pulumi/pkg/v3/backend/secrets"
	cmdConfig "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/config"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/metadata"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
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
// The stack is opened by the CLI layer and passed through here so the hook doesn't repeat the
// load; Snippet.UUID carries a freshly minted proposal that the engine resolves against any
// existing snippet of the same Name+Type.
type StatefulUpdateRequest struct {
	Snippet      resource.Snippet
	Stack        backend.Stack
	DryRun       bool
	Yes          bool
	ShowSecrets  bool
	Delete       bool
	RequireFresh bool
	Proj         *workspace.Project
	Root         string
	Sink         diag.Sink
}

// StatefulUpdateResult carries whatever the caller wants to render after the update. For the first
// cut it is empty — the resource's post-update outputs will be plumbed through once we're reading
// them out of the returned snapshot.
type StatefulUpdateResult struct{}

// RunStatefulUpdateFunc is the injection point for driving the backend update/preview operation.
// NewDoCmd assigns the default implementation (real backend + engine); tests substitute a capturing
// stub so the CLI-level construction of the snippet and target can be exercised without a live
// backend.
//
// The stack is passed in via req rather than looked up here — the CLI layer holds the stack open
// and hands it through.
type RunStatefulUpdateFunc func(
	ctx context.Context, flags *pflag.FlagSet, req StatefulUpdateRequest,
) (*StatefulUpdateResult, error)

func (pc *packageCommand) newStatefulResourceUpsertCommand(res *schema.Resource) *cobra.Command {
	var inputFile string
	var inputFormat string
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
				res:          res,
				name:         args[0],
				inputFile:    inputFile,
				inputFormat:  inputFormat,
				yes:          yes,
				verb:         "upserted",
				requireFresh: false,
			})
		},
	}
	addStatefulSnippetUpdateFlags(cmd, &inputFile, &inputFormat, &yes, res.InputProperties)
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
				collectInputFlags(cmd, "input", res.InputProperties))
			if err != nil {
				return fmt.Errorf("parse input file: %w", err)
			}
			if read.Outputs == nil {
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
// that vary between commands. Everything else — parsing the input file, loading the stack, and
// dispatching to runStatefulUpdate — is shared.
type statefulSnippetUpdate struct {
	res         *schema.Resource
	name        string
	inputFile   string
	inputFormat string
	yes         bool
	verb        string // completion-message verb, e.g. "created" or "upserted"
	// requireFresh errors when a snippet with the same (Name, Type) already exists in the stack —
	// the invariant `create` enforces to distinguish itself from `upsert`.
	requireFresh bool
}

// runStatefulSnippetUpdate is the shared body of `create` (with requireFresh=true) and `upsert`
// (with requireFresh=false). Both take the same inputs, differ only in the policy check
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

	// Merge --input-* flags into the file's PCL AST so the persisted snippet body matches what
	// the user typed on the command line. If no file was provided, the flags become the snippet
	// body by themselves.
	inputFlags := collectInputFlags(cmd, "input", args.res.InputProperties)
	code, _, err := parseFile(
		ctx, args.inputFile, "input", args.inputFormat, args.res.Token,
		pc.converter, pc.loaderTarget, pc.packageDescriptor, inputFlags,
	)
	if err != nil {
		return fmt.Errorf("read input file: %w", err)
	}

	displayOpts := display.Options{Color: cmdutil.GetGlobalColorization()}
	stack, err := cmdStack.RequireStack(
		ctx, pc.diagFwd, pc.ws, pc.lm,
		"",                                 /*stackName — use currently selected*/
		cmdStack.LoadOnly, displayOpts, "", /*configFile*/
	)
	if err != nil {
		return fmt.Errorf("load stack: %w", err)
	}

	proposed, err := uuid.NewV4()
	if err != nil {
		return fmt.Errorf("generate snippet uuid: %w", err)
	}
	snippet := resource.Snippet{
		UUID:       proposed.String(),
		Name:       args.name,
		Type:       args.res.Token,
		Code:       string(code),
		Descriptor: packageDescriptorFromProto(pc.packageDescriptor),
	}

	result, err := pc.runStatefulUpdate(ctx, cmd.Flags(), StatefulUpdateRequest{
		Snippet:      snippet,
		Stack:        stack,
		DryRun:       pc.dryrun,
		Yes:          args.yes,
		ShowSecrets:  pc.showSecrets,
		RequireFresh: args.requireFresh,
		Proj:         pc.proj,
		Root:         pc.root,
		Sink:         pc.diagFwd,
	})
	if err != nil {
		if errors.Is(err, engine.ErrSnippetExists) {
			return fmt.Errorf("resource %s %q already exists in stack %s; use `upsert` to replace it",
				args.res.Token, args.name, stack.Ref())
		}
		return err
	}
	if result != nil && !pc.dryrun {
		fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", args.verb, args.name)
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

	proposed, err := uuid.NewV4()
	if err != nil {
		return fmt.Errorf("generate snippet uuid: %w", err)
	}

	result, err := pc.runStatefulUpdate(ctx, cmd.Flags(), StatefulUpdateRequest{
		Snippet: resource.Snippet{
			UUID: proposed.String(),
			Name: name,
			Type: res.Token,
		},
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
		if errors.Is(err, engine.ErrSnippetNotFound) {
			return fmt.Errorf("resource %s %q does not exist in stack %s", res.Token, name, stack.Ref())
		}
		return err
	}
	if result != nil && !pc.dryrun {
		fmt.Fprintf(cmd.OutOrStdout(), "deleted %s\n", name)
	}
	return nil
}

// addStatefulSnippetUpdateFlags installs the flag set shared by stateful `create` and `upsert`.
func addStatefulSnippetUpdateFlags(
	cmd *cobra.Command, inputFile, inputFormat *string, yes *bool, inputs []*schema.Property,
) {
	cmd.Flags().StringVar(inputFile, "input-file", "", "Path to a file containing resource inputs")
	cmd.Flags().StringVar(inputFormat, "input", "yaml",
		"Format of the resource inputs file (any language name supported by an installed converter)")
	cmd.Flags().BoolVar(yes, "yes", false,
		"Automatically approve and perform the operation without a confirmation prompt")
	addInputFlags(cmd, "input", inputs)
}

// DefaultRunStatefulUpdate is the production implementation of the runStatefulUpdate hook. The
// caller (typically the upsert command) has already loaded the stack and proposed the snippet's
// UUID; this function loads config + secrets and calls the backend preview/update entrypoint with
// an UpdateOperation whose engine options carry the snippet and target it.
func DefaultRunStatefulUpdate(
	ctx context.Context, flags *pflag.FlagSet, req StatefulUpdateRequest,
) (*StatefulUpdateResult, error) {
	if req.Stack == nil {
		return nil, errors.New("stateful update requires a loaded stack")
	}
	displayOpts := display.Options{
		Color:       cmdutil.GetGlobalColorization(),
		ShowSecrets: req.ShowSecrets,
	}

	ssml := cmdStack.NewStackSecretsManagerLoaderFromEnv()
	cfg, sm, err := cmdConfig.GetStackConfiguration(ctx, req.Sink, ssml, req.Stack, req.Proj, "", nil)
	if err != nil {
		return nil, fmt.Errorf("get stack configuration: %w", err)
	}

	m, err := metadata.GetUpdateMetadata("", req.Root, "", "", false, cfg, flags)
	if err != nil {
		return nil, fmt.Errorf("gathering environment metadata: %w", err)
	}
	cmdutil.SetStringSpanAttributes(ctx, m.Environment)

	engineOpts := engine.UpdateOptions{
		Snippets: []engine.SnippetUpdate{{
			Snippet:      req.Snippet,
			Delete:       req.Delete,
			RequireFresh: req.RequireFresh,
		}},
		TargetSnippets: []string{req.Snippet.UUID},
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
		_, _, err = backend.PreviewStack(ctx, req.Stack, op, nil /* events */)
	} else {
		_, err = backend.UpdateStack(ctx, req.Stack, op, nil /* events */)
	}
	if err != nil {
		return nil, err
	}

	return &StatefulUpdateResult{}, nil
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
