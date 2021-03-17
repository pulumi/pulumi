//nolint:golint
package lifecycletest

import (
	"context"
	"reflect"
	"testing"

	"github.com/mitchellh/copystructure"
	"github.com/stretchr/testify/assert"

	. "github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/pkg/v3/util/cancel"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
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
	return ""
}

func (u *updateInfo) GetProject() *workspace.Project {
	return &u.project
}

func (u *updateInfo) GetTarget() *deploy.Target {
	return &u.target
}

func ImportOp(imports []deploy.Import) TestOp {
	return TestOp(func(info UpdateInfo, ctx *Context, opts UpdateOptions, dryRun bool) (ResourceChanges, result.Result) {
		return Import(info, ctx, opts, imports, dryRun)
	})
}

type TestOp func(UpdateInfo, *Context, UpdateOptions, bool) (ResourceChanges, result.Result)

type ValidateFunc func(project workspace.Project, target deploy.Target, entries JournalEntries,
	events []Event, res result.Result) result.Result

func (op TestOp) Run(project workspace.Project, target deploy.Target, opts UpdateOptions,
	dryRun bool, backendClient deploy.BackendClient, validate ValidateFunc) (*deploy.Snapshot, result.Result) {

	return op.RunWithContext(context.Background(), project, target, opts, dryRun, backendClient, validate)
}

func (op TestOp) RunWithContext(
	callerCtx context.Context, project workspace.Project,
	target deploy.Target, opts UpdateOptions, dryRun bool,
	backendClient deploy.BackendClient, validate ValidateFunc) (*deploy.Snapshot, result.Result) {

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

	// Begin draining events.
	var firedEvents []Event
	go func() {
		for e := range events {
			firedEvents = append(firedEvents, e)
		}
	}()

	// Run the step and its validator.
	_, res := op(info, ctx, opts, dryRun)
	contract.IgnoreClose(journal)

	if dryRun {
		return nil, res
	}
	if validate != nil {
		res = validate(project, target, journal.Entries(), firedEvents, res)
	}

	snap := journal.Snap(target.Snapshot)
	if res == nil && snap != nil {
		res = result.WrapIfNonNil(snap.VerifyIntegrity())
	}
	return snap, res
}

type TestStep struct {
	Op            TestOp
	ExpectFailure bool
	SkipPreview   bool
	Validate      ValidateFunc
}

type TestPlan struct {
	Project        string
	Stack          string
	Runtime        string
	RuntimeOptions map[string]interface{}
	Config         config.Map
	Decrypter      config.Decrypter
	BackendClient  deploy.BackendClient
	Options        UpdateOptions
	Steps          []TestStep
}

//nolint: goconst
func (p *TestPlan) getNames() (stack tokens.QName, project tokens.PackageName, runtime string) {
	project = tokens.PackageName(p.Project)
	if project == "" {
		project = "test"
	}
	runtime = p.Runtime
	if runtime == "" {
		runtime = "test"
	}
	stack = tokens.QName(p.Stack)
	if stack == "" {
		stack = "test"
	}
	return stack, project, runtime
}

func (p *TestPlan) NewURN(typ tokens.Type, name string, parent resource.URN) resource.URN {
	stack, project, _ := p.getNames()
	var pt tokens.Type
	if parent != "" {
		pt = parent.Type()
	}
	return resource.NewURN(stack, project, pt, typ, tokens.QName(name))
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

func (p *TestPlan) GetTarget(snapshot *deploy.Snapshot) deploy.Target {
	stack, _, _ := p.getNames()

	cfg := p.Config
	if cfg == nil {
		cfg = config.Map{}
	}

	return deploy.Target{
		Name:      stack,
		Config:    cfg,
		Decrypter: p.Decrypter,
		Snapshot:  snapshot,
	}
}

func assertIsErrorOrBailResult(t *testing.T, res result.Result) {
	assert.NotNil(t, res)
}

// CloneSnapshot makes a deep copy of the given snapshot and returns a pointer to the clone.
func CloneSnapshot(t *testing.T, snap *deploy.Snapshot) *deploy.Snapshot {
	t.Helper()
	if snap != nil {
		copiedSnap := copystructure.Must(copystructure.Copy(*snap)).(deploy.Snapshot)
		assert.True(t, reflect.DeepEqual(*snap, copiedSnap))
		return &copiedSnap
	}

	return snap
}

func (p *TestPlan) Run(t *testing.T, snapshot *deploy.Snapshot) *deploy.Snapshot {
	project := p.GetProject()
	snap := snapshot
	for _, step := range p.Steps {
		// note: it's really important that the preview and update operate on different snapshots.  the engine can and
		// does mutate the snapshot in-place, even in previews, and sharing a snapshot between preview and update can
		// cause state changes from the preview to persist even when doing an update.
		if !step.SkipPreview {
			previewSnap := CloneSnapshot(t, snap)
			previewTarget := p.GetTarget(previewSnap)
			_, res := step.Op.Run(project, previewTarget, p.Options, true, p.BackendClient, step.Validate)
			if step.ExpectFailure {
				assertIsErrorOrBailResult(t, res)
				continue
			}

			assert.Nil(t, res)
		}

		var res result.Result
		target := p.GetTarget(snap)
		snap, res = step.Op.Run(project, target, p.Options, false, p.BackendClient, step.Validate)
		if step.ExpectFailure {
			assertIsErrorOrBailResult(t, res)
			continue
		}

		if res != nil {
			if res.IsBail() {
				t.Logf("Got unexpected bail result")
				t.FailNow()
			} else {
				t.Logf("Got unexpected error result: %v", res.Error())
				t.FailNow()
			}
		}

		assert.Nil(t, res)
	}

	return snap
}

func MakeBasicLifecycleSteps(t *testing.T, resCount int) []TestStep {
	return []TestStep{
		// Initial update
		{
			Op: Update,
			Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
				_ []Event, res result.Result) result.Result {

				// Should see only creates.
				for _, entry := range entries {
					assert.Equal(t, deploy.OpCreate, entry.Step.Op())
				}
				assert.Len(t, entries.Snap(target.Snapshot).Resources, resCount)
				return res
			},
		},
		// No-op refresh
		{
			Op: Refresh,
			Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
				_ []Event, res result.Result) result.Result {

				// Should see only refresh-sames.
				for _, entry := range entries {
					assert.Equal(t, deploy.OpRefresh, entry.Step.Op())
					assert.Equal(t, deploy.OpSame, entry.Step.(*deploy.RefreshStep).ResultOp())
				}
				assert.Len(t, entries.Snap(target.Snapshot).Resources, resCount)
				return res
			},
		},
		// No-op update
		{
			Op: Update,
			Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
				_ []Event, res result.Result) result.Result {

				// Should see only sames.
				for _, entry := range entries {
					assert.Equal(t, deploy.OpSame, entry.Step.Op())
				}
				assert.Len(t, entries.Snap(target.Snapshot).Resources, resCount)
				return res
			},
		},
		// No-op refresh
		{
			Op: Refresh,
			Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
				_ []Event, res result.Result) result.Result {

				// Should see only refresh-sames.
				for _, entry := range entries {
					assert.Equal(t, deploy.OpRefresh, entry.Step.Op())
					assert.Equal(t, deploy.OpSame, entry.Step.(*deploy.RefreshStep).ResultOp())
				}
				assert.Len(t, entries.Snap(target.Snapshot).Resources, resCount)
				return res
			},
		},
		// Destroy
		{
			Op: Destroy,
			Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
				_ []Event, res result.Result) result.Result {

				// Should see only deletes.
				for _, entry := range entries {
					switch entry.Step.Op() {
					case deploy.OpDelete, deploy.OpReadDiscard:
						// ok
					default:
						assert.Fail(t, "expected OpDelete or OpReadDiscard")
					}
				}
				assert.Len(t, entries.Snap(target.Snapshot).Resources, 0)
				return res
			},
		},
		// No-op refresh
		{
			Op: Refresh,
			Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
				_ []Event, res result.Result) result.Result {

				assert.Len(t, entries, 0)
				assert.Len(t, entries.Snap(target.Snapshot).Resources, 0)
				return res
			},
		},
	}
}
