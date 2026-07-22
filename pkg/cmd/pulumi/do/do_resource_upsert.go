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
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
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
// The stack is opened and its snapshot inspected by the CLI layer (so it can resolve the snippet's
// UUID against any existing snippet of the same Name+Type); the resolved Stack is passed through
// here so the hook doesn't repeat the load.
type StatefulUpdateRequest struct {
	Snippet     resource.Snippet
	Stack       backend.Stack
	DryRun      bool
	Yes         bool
	ShowSecrets bool
	Delete      bool
	Proj        *workspace.Project
	Root        string
	Sink        diag.Sink
}

// StatefulUpdateResult carries whatever the caller wants to render after the update. For the first
// cut we only echo the snippet identity — the resource's post-update outputs will be plumbed
// through once we're reading them out of the returned snapshot.
type StatefulUpdateResult struct {
	SnippetUUID string
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

func (pc *packageCommand) newResourceUpsertCommand(res *schema.Resource) *cobra.Command {
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
		Args:   cobra.ExactArgs(1),
		Hidden: pc.stateless,
		RunE: func(cmd *cobra.Command, args []string) error {
			if pc.stateless {
				return errors.New("`upsert` is not supported in stateless mode; remove --stateless, " +
					"or use `create` or `patch` to call the provider directly")
			}
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
	if pc.stateless {
		cmd.Args = cobra.ArbitraryArgs
		cmd.FParseErrWhitelist = cobra.FParseErrWhitelist{UnknownFlags: true}
	}
	addStatefulSnippetUpdateFlags(cmd, &inputFile, &inputFormat, &yes, res.InputProperties)
	return cmd
}

// statefulSnippetUpdate carries the pieces of a stateful snippet-add operation (create / upsert)
// that vary between commands. Everything else — parsing the input file, loading the stack,
// resolving the snippet UUID, and dispatching to runStatefulUpdate — is shared.
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

	// Open the stack up front so we can look at the existing snapshot before deciding whether
	// this operation is legal (create requires a fresh snippet, upsert accepts either). The
	// stack is threaded through to runStatefulUpdate so it doesn't re-load.
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
	snippet := resource.Snippet{
		UUID:       snippetUUID,
		Name:       args.name,
		Type:       args.res.Token,
		Code:       string(code),
		Descriptor: packageDescriptorFromProto(pc.packageDescriptor),
	}

	result, err := pc.runStatefulUpdate(ctx, cmd.Flags(), StatefulUpdateRequest{
		Snippet:     snippet,
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
	if result != nil && !pc.dryrun {
		fmt.Fprintf(cmd.OutOrStdout(), "%s %s (snippet %s)\n", args.verb, args.name, result.SnippetUUID)
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
		Snippet: resource.Snippet{
			UUID: snippetUUID,
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
		return err
	}
	if result != nil && !pc.dryrun {
		fmt.Fprintf(cmd.OutOrStdout(), "deleted %s (snippet %s)\n", name, result.SnippetUUID)
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
		Color:       cmdutil.GetGlobalColorization(),
		ShowSecrets: req.ShowSecrets,
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

	snippetUUIDVal, err := uuid.FromString(req.Snippet.UUID)
	if err != nil {
		return nil, fmt.Errorf("snippet uuid: %w", err)
	}
	var snippet *resource.Snippet
	if !req.Delete {
		snippet = &req.Snippet
	}

	engineOpts := engine.UpdateOptions{
		Snippets:       map[uuid.UUID]*resource.Snippet{snippetUUIDVal: snippet},
		TargetSnippets: []string{snippetUUIDVal.String()},
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

	return &StatefulUpdateResult{SnippetUUID: req.Snippet.UUID}, nil
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
