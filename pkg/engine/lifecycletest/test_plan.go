//nolint:revive
package lifecycletest

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/blang/semver"
	"github.com/mitchellh/copystructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/display"
	. "github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/pkg/v3/util/cancel"
	"github.com/pulumi/pulumi/sdk/v3/go/common/promise"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type updateInfo struct {
	project workspace.Project
	target  deploy.Target
}

func (u *updateInfo) GetRoot() string {
	// These tests run in-memory, so we don't have a real root. Just pretend we're at the filesystem root.
	return "/"
}

func (u *updateInfo) GetProject() *workspace.Project {
	return &u.project
}

func (u *updateInfo) GetTarget() *deploy.Target {
	return &u.target
}

func ImportOp(imports []deploy.Import) TestOp {
	return TestOp(func(info UpdateInfo, ctx *Context, opts UpdateOptions,
		dryRun bool,
	) (*deploy.Plan, display.ResourceChanges, error) {
		return Import(info, ctx, opts, imports, dryRun)
	})
}

type TestOp func(UpdateInfo, *Context, UpdateOptions, bool) (*deploy.Plan, display.ResourceChanges, error)

type ValidateFunc func(project workspace.Project, target deploy.Target, entries JournalEntries,
	events []Event, err error) error

func (op TestOp) Plan(project workspace.Project, target deploy.Target, opts TestUpdateOptions,
	backendClient deploy.BackendClient, validate ValidateFunc,
) (*deploy.Plan, error) {
	plan, _, err := op.runWithContext(context.Background(), project, target, opts, true, backendClient, validate)
	return plan, err
}

func (op TestOp) Run(project workspace.Project, target deploy.Target, opts TestUpdateOptions,
	dryRun bool, backendClient deploy.BackendClient, validate ValidateFunc,
) (*deploy.Snapshot, error) {
	return op.RunWithContext(context.Background(), project, target, opts, dryRun, backendClient, validate)
}

func (op TestOp) RunWithContext(
	callerCtx context.Context, project workspace.Project,
	target deploy.Target, opts TestUpdateOptions, dryRun bool,
	backendClient deploy.BackendClient, validate ValidateFunc,
) (*deploy.Snapshot, error) {
	_, snap, err := op.runWithContext(callerCtx, project, target, opts, dryRun, backendClient, validate)
	return snap, err
}

func (op TestOp) runWithContext(
	callerCtx context.Context, project workspace.Project,
	target deploy.Target, opts TestUpdateOptions, dryRun bool,
	backendClient deploy.BackendClient, validate ValidateFunc,
) (*deploy.Plan, *deploy.Snapshot, error) {
	// Create an appropriate update info and context.
	info := &updateInfo{project: project, target: target}

	cancelCtx, cancelSrc := cancel.NewContext(context.Background())
	done := make(chan bool)
	defer close(done)
	go func() {
		select {
		case <-callerCtx.Done():
			cancelSrc.Cancel()
		case <-done:
		}
	}()

	events := make(chan Event)
	journal := NewJournal()

	ctx := &Context{
		Cancel:          cancelCtx,
		Events:          events,
		SnapshotManager: journal,
		BackendClient:   backendClient,
	}

	updateOpts := opts.Options()
	defer func() {
		if updateOpts.Host != nil {
			contract.IgnoreClose(updateOpts.Host)
		}
	}()

	// Begin draining events.
	firedEventsPromise := promise.Run(func() ([]Event, error) {
		var firedEvents []Event
		for e := range events {
			firedEvents = append(firedEvents, e)
		}
		return firedEvents, nil
	})

	// Run the step and its validator.
	plan, _, opErr := op(info, ctx, updateOpts, dryRun)
	close(events)
	closeErr := journal.Close()

	// Wait for the events to finish. You'd think this would cancel with the callerCtx but tests explicitly use that for
	// the deployment context, not expecting it to have any effect on the test code here. See
	// https://github.com/pulumi/pulumi/issues/14588 for what happens if you try to use callerCtx here.
	firedEvents, err := firedEventsPromise.Result(context.Background())
	if err != nil {
		return nil, nil, err
	}

	if validate != nil {
		opErr = validate(project, target, journal.Entries(), firedEvents, opErr)
	}
	errs := []error{opErr, closeErr}
	if dryRun {
		return plan, nil, errors.Join(errs...)
	}

	entries := journal.Entries()
	// Check that each possible snapshot we could have created is valid
	var snap *deploy.Snapshot
	for i := 0; i <= len(entries); i++ {
		var err error
		snap, err = entries[0:i].Snap(target.Snapshot)
		if err != nil {
			// if any snapshot fails to create just return this error, don't keep going
			errs = append(errs, err)
			snap = nil
			break
		}
		err = snap.VerifyIntegrity()
		if err != nil {
			// Likewise as soon as one snapshot fails to validate stop checking
			errs = append(errs, err)
			snap = nil
			break
		}
	}

	return nil, snap, errors.Join(errs...)
}

type TestStep struct {
	Op            TestOp
	ExpectFailure bool
	SkipPreview   bool
	Validate      ValidateFunc
}

func (t *TestStep) ValidateAnd(f ValidateFunc) {
	o := t.Validate
	t.Validate = func(project workspace.Project, target deploy.Target, entries JournalEntries,
		events []Event, err error,
	) error {
		r := o(project, target, entries, events, err)
		if r != nil {
			return r
		}
		return f(project, target, entries, events, err)
	}
}

// TestUpdateOptions is UpdateOptions for a TestPlan.
type TestUpdateOptions struct {
	UpdateOptions
	// a factory to produce a plugin host for an update operation.
	HostF deploytest.PluginHostFactory
}

// Options produces UpdateOptions for an update operation.
func (o TestUpdateOptions) Options() UpdateOptions {
	opts := o.UpdateOptions
	if o.HostF != nil {
		opts.Host = o.HostF()
	}
	return opts
}

type TestPlan struct {
	Project        string
	Stack          string
	Runtime        string
	RuntimeOptions map[string]interface{}
	Config         config.Map
	Decrypter      config.Decrypter
	BackendClient  deploy.BackendClient
	Options        TestUpdateOptions
	Steps          []TestStep
}

func (p *TestPlan) getNames() (stack tokens.StackName, project tokens.PackageName, runtime string) {
	project = tokens.PackageName(p.Project)
	if project == "" {
		project = "test"
	}
	runtime = p.Runtime
	if runtime == "" {
		runtime = "test"
	}
	stack = tokens.MustParseStackName("test")
	if p.Stack != "" {
		stack = tokens.MustParseStackName(p.Stack)
	}
	return stack, project, runtime
}

func (p *TestPlan) NewURN(typ tokens.Type, name string, parent resource.URN) resource.URN {
	stack, project, _ := p.getNames()
	var pt tokens.Type
	if parent != "" {
		pt = parent.QualifiedType()
	}
	return resource.NewURN(stack.Q(), project, pt, typ, name)
}

func (p *TestPlan) NewProviderURN(pkg tokens.Package, name string, parent resource.URN) resource.URN {
	return p.NewURN(providers.MakeProviderType(pkg), name, parent)
}

func (p *TestPlan) GetProject() workspace.Project {
	_, projectName, runtime := p.getNames()

	return workspace.Project{
		Name:    projectName,
		Runtime: workspace.NewProjectRuntimeInfo(runtime, p.RuntimeOptions),
	}
}

func (p *TestPlan) GetTarget(t testing.TB, snapshot *deploy.Snapshot) deploy.Target {
	stack, _, _ := p.getNames()

	cfg := p.Config
	if cfg == nil {
		cfg = config.Map{}
	}

	return deploy.Target{
		Name:      stack,
		Config:    cfg,
		Decrypter: p.Decrypter,
		// note: it's really important that the preview and update operate on different snapshots.  the engine can and
		// does mutate the snapshot in-place, even in previews, and sharing a snapshot between preview and update can
		// cause state changes from the preview to persist even when doing an update.
		Snapshot: CloneSnapshot(t, snapshot),
	}
}

// CloneSnapshot makes a deep copy of the given snapshot and returns a pointer to the clone.
func CloneSnapshot(t testing.TB, snap *deploy.Snapshot) *deploy.Snapshot {
	t.Helper()
	if snap != nil {
		copiedSnap := copystructure.Must(copystructure.Copy(*snap)).(deploy.Snapshot)
		assert.True(t, reflect.DeepEqual(*snap, copiedSnap))
		return &copiedSnap
	}

	return snap
}

func (p *TestPlan) Run(t testing.TB, snapshot *deploy.Snapshot) *deploy.Snapshot {
	project := p.GetProject()
	snap := snapshot
	for _, step := range p.Steps {
		// note: it's really important that the preview and update operate on different snapshots.  the engine can and
		// does mutate the snapshot in-place, even in previews, and sharing a snapshot between preview and update can
		// cause state changes from the preview to persist even when doing an update.
		// GetTarget ALWAYS clones the snapshot, so the previewTarget.Snapshot != target.Snapshot
		if !step.SkipPreview {
			previewTarget := p.GetTarget(t, snap)
			// Don't run validate on the preview step
			_, err := step.Op.Run(project, previewTarget, p.Options, true, p.BackendClient, nil)
			if step.ExpectFailure {
				assert.Error(t, err)
				continue
			}

			assert.NoError(t, err)
		}

		var err error
		target := p.GetTarget(t, snap)
		snap, err = step.Op.Run(project, target, p.Options, false, p.BackendClient, step.Validate)
		if step.ExpectFailure {
			assert.Error(t, err)
			continue
		}

		if err != nil {
			if result.IsBail(err) {
				t.Logf("Got unexpected bail result: %v", err)
				t.FailNow()
			} else {
				t.Logf("Got unexpected error result: %v", err)
				t.FailNow()
			}
		}

		assert.NoError(t, err)
	}

	return snap
}

// resCount is the expected number of resources registered during this test.
func MakeBasicLifecycleSteps(t *testing.T, resCount int) []TestStep {
	return []TestStep{
		// Initial update
		{
			Op: Update,
			Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
				_ []Event, err error,
			) error {
				require.NoError(t, err)

				// Should see only creates or reads.
				for _, entry := range entries {
					op := entry.Step.Op()
					assert.True(t, op == deploy.OpCreate || op == deploy.OpRead)
				}
				snap, err := entries.Snap(target.Snapshot)
				require.NoError(t, err)
				assert.Len(t, snap.Resources, resCount)
				return err
			},
		},
		// No-op refresh
		{
			Op: Refresh,
			Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
				_ []Event, err error,
			) error {
				require.NoError(t, err)

				// Should see only refresh-sames.
				for _, entry := range entries {
					assert.Equal(t, deploy.OpRefresh, entry.Step.Op())
					assert.Equal(t, deploy.OpSame, entry.Step.(*deploy.RefreshStep).ResultOp())
				}
				snap, err := entries.Snap(target.Snapshot)
				require.NoError(t, err)
				assert.Len(t, snap.Resources, resCount)
				return err
			},
		},
		// No-op update
		{
			Op: Update,
			Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
				_ []Event, err error,
			) error {
				require.NoError(t, err)

				// Should see only sames.
				for _, entry := range entries {
					op := entry.Step.Op()
					assert.True(t, op == deploy.OpSame || op == deploy.OpRead)
				}
				snap, err := entries.Snap(target.Snapshot)
				require.NoError(t, err)
				assert.Len(t, snap.Resources, resCount)
				return err
			},
		},
		// No-op refresh
		{
			Op: Refresh,
			Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
				_ []Event, err error,
			) error {
				require.NoError(t, err)

				// Should see only refresh-sames.
				for _, entry := range entries {
					assert.Equal(t, deploy.OpRefresh, entry.Step.Op())
					assert.Equal(t, deploy.OpSame, entry.Step.(*deploy.RefreshStep).ResultOp())
				}
				snap, err := entries.Snap(target.Snapshot)
				require.NoError(t, err)
				assert.Len(t, snap.Resources, resCount)
				return err
			},
		},
		// Destroy
		{
			Op: Destroy,
			Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
				_ []Event, err error,
			) error {
				require.NoError(t, err)

				// Should see only deletes.
				for _, entry := range entries {
					switch entry.Step.Op() {
					case deploy.OpDelete, deploy.OpReadDiscard:
						// ok
					default:
						assert.Fail(t, "expected OpDelete or OpReadDiscard")
					}
				}
				snap, err := entries.Snap(target.Snapshot)
				require.NoError(t, err)
				assert.Len(t, snap.Resources, 0)
				return err
			},
		},
		// No-op refresh
		{
			Op: Refresh,
			Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
				_ []Event, err error,
			) error {
				require.NoError(t, err)

				assert.Len(t, entries, 0)
				snap, err := entries.Snap(target.Snapshot)
				require.NoError(t, err)
				assert.Len(t, snap.Resources, 0)
				return err
			},
		},
	}
}

type testBuilder struct {
	t       *testing.T
	loaders []*deploytest.ProviderLoader
	snap    *deploy.Snapshot
}

func newTestBuilder(t *testing.T, snap *deploy.Snapshot) *testBuilder {
	return &testBuilder{
		t:       t,
		snap:    snap,
		loaders: slice.Prealloc[*deploytest.ProviderLoader](1),
	}
}

func (b *testBuilder) WithProvider(name string, version string, prov *deploytest.Provider) *testBuilder {
	loader := deploytest.NewProviderLoader(
		tokens.Package(name), semver.MustParse(version), func() (plugin.Provider, error) {
			return prov, nil
		})
	b.loaders = append(b.loaders, loader)
	return b
}

type Result struct {
	snap *deploy.Snapshot
	err  error
}

func (b *testBuilder) RunUpdate(program func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error) *Result {
	programF := deploytest.NewLanguageRuntimeF(program)
	hostF := deploytest.NewPluginHostF(nil, nil, programF, b.loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{HostF: hostF},
	}

	// Run an update for initial state.
	var err error
	snap, err := TestOp(Update).Run(
		p.GetProject(), p.GetTarget(b.t, b.snap), p.Options, false, p.BackendClient, nil)
	return &Result{
		snap: snap,
		err:  err,
	}
}

// Then() is used to convey dependence between program runs via program structure.
func (res *Result) Then(do func(snap *deploy.Snapshot, err error)) {
	do(res.snap, res.err)
}
