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

	"github.com/pulumi/esc"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	backenddisplay "github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/diy"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	backendsecrets "github.com/pulumi/pulumi/pkg/v3/backend/secrets"
	cmdConfig "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/config"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	b64secrets "github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// Options selects and configures a stack to drive.
type Options struct {
	// BackendURL is the state backend, e.g. "file:///abs/path/to/state". When empty it
	// defaults to the backend the CLI would use -- PULUMI_BACKEND_URL, the project's
	// configured backend, or the logged-in backend -- so a caller need not restate the
	// backend it is already logged into. Only the DIY backends (file/s3/gs/azblob) are
	// supported today; the cloud backend is the named-reference (server-side) path.
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
	// cloud marks a stack on a cloud backend: config (including ESC environments) and the
	// secrets manager resolve per-operation through the CLI's own assembly.
	cloud bool
	sink  diag.Sink
	// stackCfg is the config loaded from the stack's own Pulumi.<stack>.yaml settings
	// file; the operation merges project defaults beneath it and the driver's Config
	// overlay above it (the CLI's -c semantics).
	stackCfg config.Map
}

// Result is the outcome of an Up.
type Result struct {
	Changes display.ResourceChanges
	Outputs property.Map
}

// Select opens the backend, ensures the stack exists, and returns a handle to drive it.
func Select(ctx context.Context, opts Options) (*Stack, error) {
	if opts.WorkDir == "" || opts.Stack == "" {
		return nil, fmt.Errorf("WorkDir and Stack are both required")
	}
	// The engine requires an absolute program root (ProgramInfo panics on a relative one), so
	// resolve WorkDir up front, as the CLI resolves a project directory.
	workDir, err := filepath.Abs(opts.WorkDir)
	if err != nil {
		return nil, fmt.Errorf("resolving WorkDir %q: %w", opts.WorkDir, err)
	}
	opts.WorkDir = workDir
	sink := diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})

	proj, err := workspace.LoadProject(filepath.Join(opts.WorkDir, "Pulumi.yaml"))
	if err != nil {
		return nil, fmt.Errorf("loading project at %s: %w", opts.WorkDir, err)
	}

	// Resolve the backend: an explicit URL wins, otherwise default to the backend the CLI
	// would use, so a caller need not restate the backend it is already logged into.
	backendURL := opts.BackendURL
	if backendURL == "" {
		backendURL, err = pkgWorkspace.GetCurrentCloudURL(pkgWorkspace.Instance, env.Global(), proj)
		if err != nil {
			return nil, fmt.Errorf("resolving the current backend: %w", err)
		}
		if backendURL == "" {
			return nil, fmt.Errorf("no backend selected: run `pulumi login` or set PULUMI_BACKEND_URL")
		}
	}
	// A DIY URL opens the file/object-store backend; anything else is a cloud backend
	// (app.pulumi.com or self-hosted), reached with the ambient login's credentials.
	cloud := !diy.IsDIYBackendURL(backendURL)
	var be backend.Backend
	if cloud {
		cb, err := httpstate.New(ctx, sink, backendURL, proj, false /*insecure*/)
		if err != nil {
			return nil, fmt.Errorf("opening cloud backend %s: %w", backendURL, err)
		}
		be = cb
	} else {
		// A file:// backend expects its state directory to exist (the CLI creates it on
		// login); create it here so the driver is self-contained.
		if path, ok := strings.CutPrefix(backendURL, "file://"); ok {
			if err := os.MkdirAll(path, 0o700); err != nil {
				return nil, fmt.Errorf("creating backend directory %s: %w", path, err)
			}
		}
		db, err := diy.New(ctx, sink, backendURL, proj)
		if err != nil {
			return nil, fmt.Errorf("opening backend %s: %w", backendURL, err)
		}
		db.SetCurrentProject(proj)
		be = db
	}

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

	// Load the stack's own settings file (Pulumi.<stack>.yaml): the per-stack config --
	// regions, sizes, endpoints -- that the program depends on exactly as the CLI would
	// resolve it. No file simply means no stack-level config. Cloud stacks skip this:
	// their configuration (settings file, service secrets manager, ESC environments)
	// assembles per-operation through the CLI's own helper.
	stackCfg := config.Map{}
	if cloud {
		return &Stack{be: be, stack: stack, proj: proj, opts: opts, sm: nil, cloud: true, sink: sink}, nil
	}
	settingsPath := filepath.Join(opts.WorkDir, "Pulumi."+ref.Name().String()+".yaml")
	if _, statErr := os.Stat(settingsPath); statErr == nil {
		ps, err := workspace.LoadProjectStack(sink, proj, settingsPath)
		if err != nil {
			return nil, fmt.Errorf("loading stack settings %s: %w", settingsPath, err)
		}
		// Two gaps are loud rather than silent: an ESC environment the driver cannot
		// resolve, and secure: values it cannot decrypt without the matching secrets
		// manager. Both are the seam markers for driving cloud-managed stacks later.
		if ps.Environment != nil {
			return nil, fmt.Errorf(
				"stack settings %s use ESC environments, which the in-process driver does not resolve yet",
				settingsPath)
		}
		for k, v := range ps.Config {
			if v.Secure() && opts.SecretsManager == nil {
				return nil, fmt.Errorf(
					"stack settings %s carry a secure value for %q; supply the stack's secrets manager "+
						"via Options.SecretsManager to decrypt it", settingsPath, k)
			}
		}
		stackCfg = ps.Config
	}

	return &Stack{be: be, stack: stack, proj: proj, opts: opts, sm: sm, stackCfg: stackCfg}, nil
}

// Preview computes the operation's plan and resource changes without applying them, and
// returns the stack's PROJECTED outputs -- what its outputs would be if the plan were applied,
// known where computable and absent where they depend on not-yet-created resources. The
// projected outputs are what let a delivery rollout's cascaded preview thread one stack's
// result into the next stack's previewed inputs.
func (s *Stack) Preview(ctx context.Context) (Result, *deploy.Plan, error) {
	op, err := s.operation(ctx, true /*preview*/)
	if err != nil {
		return Result{}, nil, err
	}
	var plan *deploy.Plan
	var changes display.ResourceChanges
	err = s.withEvents(ctx, func(events chan<- engine.Event) error {
		var e error
		plan, changes, e = backend.PreviewStack(ctx, s.stack, op, events)
		return e
	})
	if err != nil {
		return Result{}, nil, err
	}
	return Result{Changes: changes, Outputs: projectedStackOutputs(plan)}, plan, nil
}

// projectedStackOutputs extracts the root stack's projected outputs from a preview plan: the
// outputs the program registered, evaluated against the previewed state, with known values
// where computable. It returns an empty map when the plan has no root-stack entry (nothing to
// do). The plan carries these only because Preview enables GeneratePlan.
func projectedStackOutputs(plan *deploy.Plan) property.Map {
	if plan == nil {
		return property.Map{}
	}
	for u, rp := range plan.ResourcePlans {
		if u.Type() == resource.RootStackType && rp != nil && rp.Outputs != nil {
			return resource.FromResourcePropertyMap(rp.Outputs)
		}
	}
	return property.Map{}
}

// Up applies the operation, converging the stack to its program's desired state, and
// returns the resource changes and the stack's outputs.
func (s *Stack) Up(ctx context.Context) (Result, error) {
	op, err := s.operation(ctx, false /*preview*/)
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

// UpMany converges several stacks as one coordinated multistack operation. The backend derives a
// cross-stack dependency graph from the StackReferences in the members' snapshots and schedules
// them topologically -- parallel within a dependency level -- so a member that reads another's
// outputs waits for it. This is cross-stack ordering with no dependsOn: the caller lists the
// members and the engine orders them. Each stack's Result is keyed by its stack name.
func UpMany(ctx context.Context, specs []Options) (map[string]Result, error) {
	return runMany(ctx, specs, false /*preview*/)
}

// PreviewMany is UpMany's dry run: it plans all the members together, cascading cross-stack
// references, and returns their projected changes and outputs without converging anything.
func PreviewMany(ctx context.Context, specs []Options) (map[string]Result, error) {
	return runMany(ctx, specs, true /*preview*/)
}

func runMany(ctx context.Context, specs []Options, preview bool) (map[string]Result, error) {
	if len(specs) == 0 {
		return map[string]Result{}, nil
	}
	entries := make([]backend.MultistackEntry, len(specs))
	stacks := make([]*Stack, len(specs))
	for i, o := range specs {
		s, err := Select(ctx, o)
		if err != nil {
			return nil, err
		}
		op, err := s.operation(ctx, preview)
		if err != nil {
			return nil, err
		}
		entries[i] = backend.MultistackEntry{Stack: s.stack, Op: op, Dir: o.WorkDir}
		stacks[i] = s
	}

	run := backend.MultistackUpdate
	if preview {
		run = backend.MultistackPreview
	}
	results, err := run(ctx, entries, backend.MultistackOptions{
		Engine:      specs[0].Engine,
		DisplayOpts: backenddisplay.Options{Color: colors.Never, Stdout: io.Discard, Stderr: io.Discard},
	})
	if err != nil {
		return nil, err
	}

	out := make(map[string]Result, len(stacks))
	for _, s := range stacks {
		name := s.stack.Ref().Name().String()
		res := Result{}
		if r := results[string(s.stack.Ref().FullyQualifiedName())]; r != nil {
			if r.Error != nil {
				return nil, fmt.Errorf("stack %s: %w", name, r.Error)
			}
			res.Changes = r.Changes
			if preview {
				res.Outputs = projectedStackOutputs(r.Plan)
			}
		}
		if !preview {
			outs, oerr := s.Outputs(ctx)
			if oerr != nil {
				return nil, oerr
			}
			res.Outputs = outs
		}
		out[name] = res
	}
	return out, nil
}

// Destroy tears down all of the stack's resources. (The backend's destroy path does not
// surface a streaming event channel, so OnEvent does not fire for Destroy.)
func (s *Stack) Destroy(ctx context.Context) (display.ResourceChanges, error) {
	op, err := s.operation(ctx, false /*preview*/)
	if err != nil {
		return nil, err
	}
	return backend.DestroyStack(ctx, s.stack, op, nil)
}

// Outputs reads the stack's current outputs from its latest snapshot.
func (s *Stack) Outputs(ctx context.Context) (property.Map, error) {
	provider := secrets.Provider(b64secrets.Base64SecretsProvider)
	if s.cloud {
		provider = backendsecrets.DefaultProvider
	}
	return s.stack.SnapshotStackOutputs(ctx, provider)
}

// operation assembles the backend update operation shared by preview/up/destroy.
func (s *Stack) operation(ctx context.Context, preview bool) (backend.UpdateOperation, error) {
	overlay, err := parseConfig(s.opts.Config)
	if err != nil {
		return backend.UpdateOperation{}, err
	}
	if s.cloud {
		return s.cloudOperation(ctx, preview, overlay)
	}
	// Config resolves in three layers, lowest first: project defaults (Pulumi.yaml config
	// blocks), the stack's settings file, then the driver's overlay -- the same precedence
	// the CLI gives -c flags over `pulumi config`.
	cfg := config.Map{}
	for k, v := range s.stackCfg {
		cfg[k] = v
	}
	if err := workspace.ApplyProjectConfig(
		ctx, s.stack.Ref().Name().String(), s.proj, esc.Value{}, cfg, s.sm.Encrypter(),
	); err != nil {
		return backend.UpdateOperation{}, fmt.Errorf("applying project config defaults: %w", err)
	}
	for k, v := range overlay {
		cfg[k] = v
	}
	eng := s.opts.Engine
	if preview {
		// Generate a plan so the previewed per-resource outputs (the root stack's among them)
		// are available to the caller -- the basis of cascaded rollout preview.
		eng.GeneratePlan = true
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
			Engine:      eng,
		},
		StackConfiguration: backend.StackConfiguration{Config: cfg, Decrypter: s.sm.Decrypter()},
		SecretsManager:     s.sm,
		SecretsProvider:    b64secrets.Base64SecretsProvider,
		Scopes:             contextScopes,
	}, nil
}

// cloudOperation assembles an operation for a cloud-backed stack. Configuration resolves
// exactly as the CLI resolves it -- the stack settings file, the stack's own secrets
// manager (the service's, for cloud stacks), and any ESC environments the stack imports --
// then the driver's Config overlays on top with -c semantics.
func (s *Stack) cloudOperation(
	ctx context.Context, preview bool, overlay config.Map,
) (backend.UpdateOperation, error) {
	ssml := cmdStack.NewStackSecretsManagerLoaderFromEnv()
	// Name the settings file from the stack's own WorkDir: the fallback detection walks the
	// PROCESS working directory, which for an embedded driver is the parent program's
	// project, not this stack's.
	configFile := ""
	if p := filepath.Join(s.opts.WorkDir, "Pulumi."+s.stack.Ref().Name().String()+".yaml"); fileExists(p) {
		configFile = p
	}
	cfg, sm, err := cmdConfig.GetStackConfiguration(ctx, s.sink, ssml, s.stack, s.proj, configFile)
	if err != nil {
		return backend.UpdateOperation{}, fmt.Errorf("assembling stack configuration: %w", err)
	}
	// Fold the ESC environment's pulumiConfig and the project's config defaults into the
	// stack config -- the same application the CLI performs before every operation.
	if err := workspace.ValidateStackConfigAndApplyProjectConfig(
		ctx, s.stack.Ref().Name().String(), s.proj, cfg.Environment, cfg.Config,
		sm.Encrypter(), sm.Decrypter(),
	); err != nil {
		return backend.UpdateOperation{}, fmt.Errorf("validating stack config: %w", err)
	}
	for k, v := range overlay {
		cfg.Config[k] = v
	}
	eng := s.opts.Engine
	if preview {
		eng.GeneratePlan = true
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
			Engine:      eng,
		},
		StackConfiguration: cfg,
		SecretsManager:     sm,
		SecretsProvider:    backendsecrets.DefaultProvider,
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

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
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
