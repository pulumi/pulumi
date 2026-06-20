// Copyright 2026, Pulumi Corporation.

// Package auto is an in-process, statically-linked driver for Pulumi stack operations --
// "Automation API v2". Where sdk/go/auto shells out to the `pulumi` CLI, this links the
// engine and backend directly and drives preview/up/destroy/outputs as ordinary library
// calls. It is the substrate a server-side runner or an orchestration provider (e.g.
// Pulumi Delivery's Deployment[stack]) uses to converge a child stack without spawning a
// `pulumi` process. The engine still launches language and provider plugins as
// subprocesses -- that is inherent to Pulumi -- but there is no CLI in the loop.
//
// The single substantive difference from the CLI's own update path is the cancellation
// source: the CLI installs a process-wide SIGINT/SIGTERM handler, which a nested driver
// running inside another process (a provider, a runner) must never do. This package
// derives cancellation purely from the caller's context instead (see contextScopeSource).
package auto

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	backenddisplay "github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/diy"
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	b64secrets "github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// Options selects and configures a stack to drive.
type Options struct {
	// BackendURL is the state backend, e.g. "file:///abs/path/to/state". Required. Only
	// the DIY backends (file/s3/gs/azblob) are supported today; the cloud backend is a
	// follow-on.
	BackendURL string
	// WorkDir is the stack's project directory -- it must contain a Pulumi.yaml. Required.
	WorkDir string
	// Stack is the stack name to operate on; it is created if it does not yet exist.
	Stack string
	// Config is plaintext configuration to apply for the operation, keyed by full config
	// key ("namespace:name", e.g. "aws:region"). v1 carries no secret config.
	Config map[string]string
	// SecretsManager overrides the secrets manager used for the stack's state. Defaults to
	// a base64 manager -- adequate for hermetic stacks; a passphrase/cloud manager is a
	// follow-on for real secrets.
	SecretsManager secrets.Manager
	// OnEvent, if set, receives engine events as the operation streams. The driver never
	// writes to stdout/stderr itself; consume events here to render progress.
	OnEvent func(engine.Event)
	// Engine overrides engine update options (parallelism, targets, plugin host, ...). A
	// nil Host lets the engine resolve language and provider plugins from PATH as usual.
	Engine engine.UpdateOptions
}

// Stack is a selected stack, ready to be previewed, updated, destroyed, or read.
type Stack struct {
	be    backend.Backend
	stack backend.Stack
	proj  *workspace.Project
	opts  Options
	sm    secrets.Manager
}

// Result is the outcome of an Up.
type Result struct {
	Changes display.ResourceChanges
	Outputs property.Map
}

// Select opens the backend, ensures the stack exists, and returns a handle to drive it.
func Select(ctx context.Context, opts Options) (*Stack, error) {
	if opts.BackendURL == "" || opts.WorkDir == "" || opts.Stack == "" {
		return nil, fmt.Errorf("BackendURL, WorkDir, and Stack are all required")
	}
	sink := diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})

	proj, err := workspace.LoadProject(filepath.Join(opts.WorkDir, "Pulumi.yaml"))
	if err != nil {
		return nil, fmt.Errorf("loading project at %s: %w", opts.WorkDir, err)
	}

	// A file:// backend expects its state directory to exist (the CLI creates it on
	// login); create it here so the driver is self-contained.
	if path, ok := strings.CutPrefix(opts.BackendURL, "file://"); ok {
		if err := os.MkdirAll(path, 0o700); err != nil {
			return nil, fmt.Errorf("creating backend directory %s: %w", path, err)
		}
	}

	be, err := diy.New(ctx, sink, opts.BackendURL, proj)
	if err != nil {
		return nil, fmt.Errorf("opening backend %s: %w", opts.BackendURL, err)
	}
	be.SetCurrentProject(proj)

	ref, err := be.ParseStackReference(opts.Stack)
	if err != nil {
		return nil, fmt.Errorf("parsing stack reference %q: %w", opts.Stack, err)
	}
	stack, err := be.GetStack(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("getting stack %q: %w", opts.Stack, err)
	}
	if stack == nil {
		stack, err = be.CreateStack(ctx, ref, opts.WorkDir, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("creating stack %q: %w", opts.Stack, err)
		}
	}

	sm := opts.SecretsManager
	if sm == nil {
		sm = b64secrets.NewBase64SecretsManager()
	}
	return &Stack{be: be, stack: stack, proj: proj, opts: opts, sm: sm}, nil
}

// Preview computes the operation's plan and resource changes without applying them.
func (s *Stack) Preview(ctx context.Context) (display.ResourceChanges, *deploy.Plan, error) {
	op, err := s.operation(true /*preview*/)
	if err != nil {
		return nil, nil, err
	}
	var plan *deploy.Plan
	var changes display.ResourceChanges
	err = s.withEvents(ctx, func(events chan<- engine.Event) error {
		var e error
		plan, changes, e = backend.PreviewStack(ctx, s.stack, op, events)
		return e
	})
	return changes, plan, err
}

// Up applies the operation, converging the stack to its program's desired state, and
// returns the resource changes and the stack's outputs.
func (s *Stack) Up(ctx context.Context) (Result, error) {
	op, err := s.operation(false /*preview*/)
	if err != nil {
		return Result{}, err
	}
	var changes display.ResourceChanges
	err = s.withEvents(ctx, func(events chan<- engine.Event) error {
		var e error
		changes, e = backend.UpdateStack(ctx, s.stack, op, events)
		return e
	})
	if err != nil {
		return Result{}, err
	}
	outs, err := s.Outputs(ctx)
	if err != nil {
		return Result{}, err
	}
	return Result{Changes: changes, Outputs: outs}, nil
}

// Destroy tears down all of the stack's resources. (The backend's destroy path does not
// surface a streaming event channel, so OnEvent does not fire for Destroy.)
func (s *Stack) Destroy(ctx context.Context) (display.ResourceChanges, error) {
	op, err := s.operation(false /*preview*/)
	if err != nil {
		return nil, err
	}
	return backend.DestroyStack(ctx, s.stack, op)
}

// Outputs reads the stack's current outputs from its latest snapshot.
func (s *Stack) Outputs(ctx context.Context) (property.Map, error) {
	return s.stack.SnapshotStackOutputs(ctx, b64secrets.Base64SecretsProvider)
}

// operation assembles the backend update operation shared by preview/up/destroy.
func (s *Stack) operation(preview bool) (backend.UpdateOperation, error) {
	cfg, err := parseConfig(s.opts.Config)
	if err != nil {
		return backend.UpdateOperation{}, err
	}
	return backend.UpdateOperation{
		Proj: s.proj,
		Root: s.opts.WorkDir,
		M:    &backend.UpdateMetadata{},
		Opts: backend.UpdateOptions{
			AutoApprove: true,
			SkipPreview: true,
			PreviewOnly: preview,
			Display:     backenddisplay.Options{Color: colors.Never, Stdout: io.Discard, Stderr: io.Discard},
			Engine:      s.opts.Engine,
		},
		StackConfiguration: backend.StackConfiguration{Config: cfg, Decrypter: s.sm.Decrypter()},
		SecretsManager:     s.sm,
		SecretsProvider:    b64secrets.Base64SecretsProvider,
		Scopes:             contextScopes,
	}, nil
}

// withEvents runs an operation against a fresh event channel, forwarding each event to the
// caller's OnEvent (if any) and waiting for the drain to finish before returning.
func (s *Stack) withEvents(ctx context.Context, run func(chan<- engine.Event) error) error {
	events := make(chan engine.Event)
	done := make(chan struct{})
	go func() {
		for e := range events {
			if s.opts.OnEvent != nil {
				s.opts.OnEvent(e)
			}
		}
		close(done)
	}()
	err := run(events)
	close(events)
	<-done
	return err
}

func parseConfig(in map[string]string) (config.Map, error) {
	cfg := config.Map{}
	for k, v := range in {
		key, err := config.ParseKey(k)
		if err != nil {
			return nil, fmt.Errorf("parsing config key %q: %w", k, err)
		}
		cfg[key] = config.NewValue(v)
	}
	return cfg, nil
}
