// Copyright 2020-2024, Pulumi Corporation.
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

package framework

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/blang/semver"
	"github.com/go-test/deep"
	"github.com/mitchellh/copystructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	bdisplay "github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	"github.com/pulumi/pulumi/pkg/v3/util/cancel"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/promise"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// TB is a subset of testing.TB that admits other T-like things, such as *rapid.T from the Rapid property-testing
// library. It covers the set of functionality that we actually need for lifecycle testing, and satisfies the interfaces
// of testify.assert and testify.require.
type TB interface {
	Helper()
	Name() string
	Logf(format string, args ...any)
	Log(args ...any)
	Skipf(format string, args ...any)
	Skip(args ...any)
	SkipNow()
	Errorf(format string, args ...any)
	Error(args ...any)
	Fatalf(format string, args ...any)
	Fatal(args ...any)
	FailNow()
	Fail()
	Failed() bool
}

func snapshotEqual(journal, manager *deploy.Snapshot) error {
	// Just want to check the same operations and resources are counted, but order might be slightly different.
	if journal == nil && manager == nil {
		return nil
	}
	if journal == nil {
		return errors.New("journal snapshot is nil")
	}
	if manager == nil {
		return errors.New("manager snapshot is nil")
	}

	// Manifests and SecretsManagers are known to differ because we don't thread them through for the Journal code.

	if len(journal.PendingOperations) != len(manager.PendingOperations) {
		return errors.New("journal and manager pending operations differ")
	}

	for _, jop := range journal.PendingOperations {
		found := false
		for _, mop := range manager.PendingOperations {
			if reflect.DeepEqual(jop, mop) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("journal and manager pending operations differ, %v not found in manager", jop)
		}
	}

	if len(journal.Resources) != len(manager.Resources) {
		var journalResources string
		for _, r := range journal.Resources {
			journalResources += fmt.Sprintf("%v %v, ", r.URN, r.Delete)
		}
		var managerResources string
		for _, r := range manager.Resources {
			managerResources += fmt.Sprintf("%v %v, ", r.URN, r.Delete)
		}
		return fmt.Errorf("journal and manager resources differ, %d in journal (have %v), %d in manager (have %v)",
			len(journal.Resources), journalResources, len(manager.Resources), managerResources)
	}

	for _, jr := range journal.Resources {
		found := false
		var diffStr string
		for _, mr := range manager.Resources {
			if diff := deep.Equal(jr, mr); diff != nil {
				if jr.URN == mr.URN {
					diffStr += fmt.Sprintf("%s\n", diff)
				}
			} else {
				found = true
				break
			}
		}
		if !found {
			var journalResources string
			for _, jr := range journal.Resources {
				journalResources += fmt.Sprintf("Journal resource: %v\n", jr)
			}
			var managerResources string
			for _, mr := range manager.Resources {
				managerResources += fmt.Sprintf("Manager resource: %v\n", mr)
			}
			return fmt.Errorf("journal and manager resources differ, %v not found in manager.\n"+
				"Journal: %v\nManager: %v\nDiffs: %v",
				jr, journalResources, managerResources, diffStr)
		}
	}

	return nil
}

func NewUpdateInfo(project workspace.Project, target deploy.Target) engine.UpdateInfo {
	return engine.UpdateInfo{
		// The tests run in-memory, so we don't have a real root. Just pretend we're at the filesystem root.
		Root:    "/",
		Project: &project,
		Target:  &target,
	}
}

func ImportOp(imports []deploy.Import) TestOp {
	return TestOp(func(info engine.UpdateInfo, ctx *engine.Context, opts engine.UpdateOptions,
		dryRun bool,
	) (*deploy.Plan, display.ResourceChanges, error) {
		return engine.Import(info, ctx, opts, imports, dryRun)
	})
}

type TestOp func(
	engine.UpdateInfo,
	*engine.Context,
	engine.UpdateOptions,
	bool,
) (*deploy.Plan, display.ResourceChanges, error)

type ValidateFunc func(project workspace.Project, target deploy.Target, entries engine.JournalEntries,
	events []engine.Event, err error) error

func (op TestOp) Plan(project workspace.Project, target deploy.Target, opts TestUpdateOptions,
	backendClient deploy.BackendClient, validate ValidateFunc,
) (*deploy.Plan, error) {
	plan, _, err := op.runWithContext(context.Background(), project, target, opts, true, backendClient, validate, "")
	return plan, err
}

func (op TestOp) PlanStep(project workspace.Project, target deploy.Target, opts TestUpdateOptions,
	backendClient deploy.BackendClient, validate ValidateFunc, name string,
) (*deploy.Plan, error) {
	plan, _, err := op.runWithContext(context.Background(), project, target, opts, true, backendClient, validate, name)
	return plan, err
}

func (op TestOp) Run(project workspace.Project, target deploy.Target, opts TestUpdateOptions,
	dryRun bool, backendClient deploy.BackendClient, validate ValidateFunc,
) (*deploy.Snapshot, error) {
	return op.RunStep(project, target, opts, dryRun, backendClient, validate, "")
}

func (op TestOp) RunStep(project workspace.Project, target deploy.Target, opts TestUpdateOptions,
	dryRun bool, backendClient deploy.BackendClient, validate ValidateFunc, name string,
) (*deploy.Snapshot, error) {
	return op.RunWithContextStep(context.Background(), project, target, opts, dryRun, backendClient, validate, name)
}

func (op TestOp) RunWithContext(
	callerCtx context.Context, project workspace.Project,
	target deploy.Target, opts TestUpdateOptions, dryRun bool,
	backendClient deploy.BackendClient, validate ValidateFunc,
) (*deploy.Snapshot, error) {
	return op.RunWithContextStep(callerCtx, project, target, opts, dryRun, backendClient, validate, "")
}

func (op TestOp) RunWithContextStep(
	callerCtx context.Context, project workspace.Project,
	target deploy.Target, opts TestUpdateOptions, dryRun bool,
	backendClient deploy.BackendClient, validate ValidateFunc, name string,
) (*deploy.Snapshot, error) {
	_, snap, err := op.runWithContext(callerCtx, project, target, opts, dryRun, backendClient, validate, name)
	return snap, err
}

func (op TestOp) runWithContext(
	callerCtx context.Context, project workspace.Project,
	target deploy.Target, opts TestUpdateOptions, dryRun bool,
	backendClient deploy.BackendClient, validate ValidateFunc, name string,
) (*deploy.Plan, *deploy.Snapshot, error) {
	// Create an appropriate update info and context.
	info := NewUpdateInfo(project, target)

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

	events := make(chan engine.Event)
	journal := engine.NewTestJournal()
	persister := &backend.InMemoryPersister{}
	secretsManager := b64.NewBase64SecretsManager()
	snapshotManager := backend.NewSnapshotManager(persister, secretsManager, target.Snapshot)

	combined := &engine.CombinedManager{
		Managers: []engine.SnapshotManager{journal, snapshotManager},
	}

	ctx := &engine.Context{
		Cancel:          cancelCtx,
		Events:          events,
		SnapshotManager: combined,
		BackendClient:   backendClient,
	}

	updateOpts := opts.Options()
	// We want to always run with plan generation to ensure that plans _can_ be generated.
	// This is to prevent regressions such as https://github.com/pulumi/pulumi/pull/19750.
	updateOpts.GeneratePlan = true
	defer func() {
		if updateOpts.Host != nil {
			contract.IgnoreClose(updateOpts.Host)
		}
	}()

	// Begin draining events.
	firedEventsPromise := promise.Run(func() ([]engine.Event, error) {
		var firedEvents []engine.Event
		for e := range events {
			firedEvents = append(firedEvents, e)
		}
		return firedEvents, nil
	})

	// Run the step and its validator.
	plan, _, opErr := op(info, ctx, updateOpts, dryRun)
	close(events)
	closeErr := combined.Close()

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

	if !opts.SkipDisplayTests {
		// base64 encode the name if it contains special characters
		if ok, err := regexp.MatchString(`^[0-9A-Za-z-_]*$`, name); !ok && name != "" {
			require.NoError(opts.T, err)
			name = base64.StdEncoding.EncodeToString([]byte(name))
			if len(name) > 64 {
				name = name[0:64]
			}
		}
		testName := opts.T.Name()
		if ok, _ := regexp.MatchString(`^[0-9A-Za-z-_]*$`, testName); !ok {
			testName = strings.ReplaceAll(testName, "[", "_")
			testName = strings.ReplaceAll(testName, "]", "_")
			testName = strings.ReplaceAll(testName, `"`, "_")
			if ok, _ := regexp.MatchString(`^[0-9A-Za-z-_]*$`, testName); !ok {
				require.NoError(opts.T, err)
				testName = base64.StdEncoding.EncodeToString([]byte(testName))
			}
		}
		AssertDisplay(opts.T, firedEvents, filepath.Join("testdata", "output", testName, name))
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

	// Verify the saved snapshot from SnapshotManger is the same(ish) as that from the Journal
	errs = append(errs, snapshotEqual(snap, persister.Snap))

	return nil, snap, errors.Join(errs...)
}

// We're just checking that we have the right number of events and
// that they have the expected types.  We don't do a deep comparison
// here, because all that matters is that we have the same events in
// some order.  The non-display tests are responsible for actually
// checking the events properly.
func compareEvents(t TB, expected, actual []engine.Event) {
	encountered := make(map[int]struct{})
	require.Len(t, actual, len(expected), "expected and actual event counts differ")
	for _, e := range expected {
		found := false
		for i, a := range actual {
			if _, ok := encountered[i]; ok {
				continue
			}
			if a.Type == e.Type {
				found = true
				encountered[i] = struct{}{}
				break
			}
		}
		assert.True(t, found, "expected event %v not found in actual events", e)
	}
	for i, e := range actual {
		if _, ok := encountered[i]; ok {
			continue
		}
		assert.Fail(t, "unexpected event %v found in actual events", e)
	}
}

func loadEvents(path string) (events []engine.Event, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening '%v': %w", path, err)
	}
	defer contract.IgnoreClose(f)

	dec := json.NewDecoder(f)
	for {
		var jsonEvent apitype.EngineEvent
		if err = dec.Decode(&jsonEvent); err != nil {
			if err == io.EOF {
				break
			}

			return nil, fmt.Errorf("decoding event %d: %w", len(events), err)
		}

		event, err := bdisplay.ConvertJSONEvent(jsonEvent)
		if err != nil {
			return nil, fmt.Errorf("converting event %d: %w", len(events), err)
		}
		events = append(events, event)
	}

	// If there are no events or if the event stream does not terminate with a cancel event,
	// synthesize one here.
	if len(events) == 0 || events[len(events)-1].Type != engine.CancelEvent {
		events = append(events, engine.NewCancelEvent())
	}

	return events, nil
}

func AssertDisplay(t TB, events []engine.Event, path string) {
	var expectedStdout []byte
	var expectedStderr []byte
	accept := cmdutil.IsTruthy(os.Getenv("PULUMI_ACCEPT"))
	if !accept {
		var err error
		expectedStdout, err = os.ReadFile(filepath.Join(path, "diff.stdout.txt"))
		require.NoError(t, err)

		expectedStderr, err = os.ReadFile(filepath.Join(path, "diff.stderr.txt"))
		require.NoError(t, err)
	}

	eventChannel, doneChannel := make(chan engine.Event), make(chan bool)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	var expectedEvents []engine.Event
	if accept {
		// Write out the events to a file for acceptance testing.
		err := os.MkdirAll(path, 0o700)
		require.NoError(t, err)

		f, err := os.OpenFile(filepath.Join(path, "eventstream.json"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		require.NoError(t, err)
		defer f.Close()

		enc := json.NewEncoder(f)
		for _, e := range events {
			apiEvent, err := bdisplay.ConvertEngineEvent(e, false)
			require.NoError(t, err)

			err = enc.Encode(apiEvent)
			require.NoError(t, err)
		}

		expectedEvents = events
	} else {
		var err error
		expectedEvents, err = loadEvents(filepath.Join(path, "eventstream.json"))
		require.NoError(t, err)

		compareEvents(t, expectedEvents, events)
	}

	// ShowProgressEvents

	go bdisplay.ShowDiffEvents("test", eventChannel, doneChannel, bdisplay.Options{
		Color:                colors.Raw,
		ShowSameResources:    true,
		ShowReplacementSteps: true,
		ShowReads:            true,
		Stdout:               &stdout,
		Stderr:               &stderr,
		DeterministicOutput:  true,
		ShowLinkToCopilot:    false,
	})

	for _, e := range expectedEvents {
		eventChannel <- e
	}
	<-doneChannel

	if !accept {
		assert.Equal(t, string(expectedStdout), stdout.String())
		assert.Equal(t, string(expectedStderr), stderr.String())
	} else {
		err := os.MkdirAll(path, 0o700)
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(path, "diff.stdout.txt"), stdout.Bytes(), 0o600)
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(path, "diff.stderr.txt"), stderr.Bytes(), 0o600)
		require.NoError(t, err)
	}

	expectedStdout = []byte{}
	expectedStderr = []byte{}
	if !accept {
		var err error
		expectedStdout, err = os.ReadFile(filepath.Join(path, "progress.stdout.txt"))
		require.NoError(t, err)

		expectedStderr, err = os.ReadFile(filepath.Join(path, "progress.stderr.txt"))
		require.NoError(t, err)
	}

	eventChannel, doneChannel = make(chan engine.Event), make(chan bool)
	stdout.Reset()
	stderr.Reset()

	go bdisplay.ShowProgressEvents(
		"test", apitype.UpdateUpdate,
		tokens.MustParseStackName("stack"), "project", "http://example.com",
		eventChannel, doneChannel, bdisplay.Options{
			Color:                colors.Raw,
			ShowSameResources:    true,
			ShowReplacementSteps: true,
			ShowReads:            true,
			SuppressProgress:     true,
			Stdout:               &stdout,
			Stderr:               &stderr,
			DeterministicOutput:  true,
			ShowLinkToCopilot:    false,
		}, false)

	for _, e := range expectedEvents {
		eventChannel <- e
	}
	<-doneChannel

	if !accept {
		assert.Equal(t, string(expectedStdout), stdout.String())
		assert.Equal(t, string(expectedStderr), stderr.String())
	} else {
		err := os.WriteFile(filepath.Join(path, "progress.stdout.txt"), stdout.Bytes(), 0o600)
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(path, "progress.stderr.txt"), stderr.Bytes(), 0o600)
		require.NoError(t, err)
	}
}

type TestStep struct {
	Op            TestOp
	ExpectFailure bool
	SkipPreview   bool
	Validate      ValidateFunc
}

func (t *TestStep) ValidateAnd(f ValidateFunc) {
	o := t.Validate
	t.Validate = func(project workspace.Project, target deploy.Target, entries engine.JournalEntries,
		events []engine.Event, err error,
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
	engine.UpdateOptions
	// a factory to produce a plugin host for an update operation.
	HostF            deploytest.PluginHostFactory
	T                TB
	SkipDisplayTests bool
}

// Options produces UpdateOptions for an update operation.
func (o TestUpdateOptions) Options() engine.UpdateOptions {
	opts := o.UpdateOptions
	if o.HostF != nil {
		opts.Host = o.HostF()
	}
	// Set a sensible parallel count because most tests leave this zero.
	if opts.Parallel == 0 {
		opts.Parallel = int32(runtime.NumCPU()) //nolint:gosec // NumCPU isn't going to overflow int32
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
	// Count the number of times Run is called on this plan.  Used to generate unique names for display snapshot tests.
	run int
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

func (p *TestPlan) GetTarget(t TB, snapshot *deploy.Snapshot) deploy.Target {
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
func CloneSnapshot(t TB, snap *deploy.Snapshot) *deploy.Snapshot {
	t.Helper()
	if snap != nil {
		copiedSnap := copystructure.Must(copystructure.Copy(*snap)).(deploy.Snapshot)
		assert.True(t, reflect.DeepEqual(*snap, copiedSnap))
		return &copiedSnap
	}

	return snap
}

func (p *TestPlan) RunWithName(t TB, snapshot *deploy.Snapshot, name string) *deploy.Snapshot {
	project := p.GetProject()
	snap := snapshot
	for i, step := range p.Steps {
		// note: it's really important that the preview and update operate on different snapshots.  the engine can and
		// does mutate the snapshot in-place, even in previews, and sharing a snapshot between preview and update can
		// cause state changes from the preview to persist even when doing an update.
		// GetTarget ALWAYS clones the snapshot, so the previewTarget.Snapshot != target.Snapshot
		if !step.SkipPreview {
			previewTarget := p.GetTarget(t, snap)
			// Don't run validate on the preview step
			_, err := step.Op.RunStep(
				project, previewTarget, p.Options, true, p.BackendClient, nil,
				fmt.Sprintf("%s-%d-%d-preview", name, i, p.run))
			if step.ExpectFailure {
				assert.Error(t, err)
				continue
			}

			require.NoError(t, err)
		}

		var err error
		target := p.GetTarget(t, snap)
		snap, err = step.Op.RunStep(project, target, p.Options, false, p.BackendClient, step.Validate,
			fmt.Sprintf("%s-%d-%d", name, i, p.run))
		if step.ExpectFailure {
			assert.Error(t, err)
			continue
		}

		errString := "unexpected error result"
		if result.IsBail(err) {
			errString = "unexpected bail result"
		}

		require.NoError(t, err, errString)
	}

	p.run++
	return snap
}

func (p *TestPlan) Run(t TB, snapshot *deploy.Snapshot) *deploy.Snapshot {
	return p.RunWithName(t, snapshot, "")
}

// resCount is the expected number of resources registered during this test.
func MakeBasicLifecycleSteps(t *testing.T, resCount int) []TestStep {
	return []TestStep{
		// Initial update
		{
			Op: engine.Update,
			Validate: func(project workspace.Project, target deploy.Target, entries engine.JournalEntries,
				_ []engine.Event, err error,
			) error {
				require.NoError(t, err)

				// Should see only creates or reads.
				for _, entry := range entries {
					op := entry.Step.Op()
					assert.True(t, op == deploy.OpCreate || op == deploy.OpRead)
				}
				snap, err := entries.Snap(target.Snapshot)
				require.NoError(t, err)
				require.Len(t, snap.Resources, resCount)
				return err
			},
		},
		// No-op refresh
		{
			Op: engine.Refresh,
			Validate: func(project workspace.Project, target deploy.Target, entries engine.JournalEntries,
				_ []engine.Event, err error,
			) error {
				require.NoError(t, err)

				// Should see only refresh-sames.
				for _, entry := range entries {
					assert.Equal(t, deploy.OpRefresh, entry.Step.Op())
					assert.Equal(t, deploy.OpSame, entry.Step.(*deploy.RefreshStep).ResultOp())
				}
				snap, err := entries.Snap(target.Snapshot)
				require.NoError(t, err)
				require.Len(t, snap.Resources, resCount)
				return err
			},
		},
		// No-op update
		{
			Op: engine.Update,
			Validate: func(project workspace.Project, target deploy.Target, entries engine.JournalEntries,
				_ []engine.Event, err error,
			) error {
				require.NoError(t, err)

				// Should see only sames.
				for _, entry := range entries {
					op := entry.Step.Op()
					assert.True(t, op == deploy.OpSame || op == deploy.OpRead)
				}
				snap, err := entries.Snap(target.Snapshot)
				require.NoError(t, err)
				require.Len(t, snap.Resources, resCount)
				return err
			},
		},
		// No-op refresh
		{
			Op: engine.Refresh,
			Validate: func(project workspace.Project, target deploy.Target, entries engine.JournalEntries,
				_ []engine.Event, err error,
			) error {
				require.NoError(t, err)

				// Should see only refresh-sames.
				for _, entry := range entries {
					assert.Equal(t, deploy.OpRefresh, entry.Step.Op())
					assert.Equal(t, deploy.OpSame, entry.Step.(*deploy.RefreshStep).ResultOp())
				}
				snap, err := entries.Snap(target.Snapshot)
				require.NoError(t, err)
				require.Len(t, snap.Resources, resCount)
				return err
			},
		},
		// Destroy
		{
			Op: engine.Destroy,
			Validate: func(project workspace.Project, target deploy.Target, entries engine.JournalEntries,
				_ []engine.Event, err error,
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
				require.Len(t, snap.Resources, 0)
				return err
			},
		},
		// No-op refresh
		{
			Op: engine.Refresh,
			Validate: func(project workspace.Project, target deploy.Target, entries engine.JournalEntries,
				_ []engine.Event, err error,
			) error {
				require.NoError(t, err)

				require.Len(t, entries, 0)
				snap, err := entries.Snap(target.Snapshot)
				require.NoError(t, err)
				require.Len(t, snap.Resources, 0)
				return err
			},
		},
	}
}

type TestBuilder struct {
	t       *testing.T
	loaders []*deploytest.ProviderLoader
	snap    *deploy.Snapshot
}

func NewTestBuilder(t *testing.T, snap *deploy.Snapshot) *TestBuilder {
	return &TestBuilder{
		t:       t,
		snap:    snap,
		loaders: slice.Prealloc[*deploytest.ProviderLoader](1),
	}
}

func (b *TestBuilder) WithProvider(name string, version string, prov *deploytest.Provider) *TestBuilder {
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

func (b *TestBuilder) RunUpdate(
	program func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error, skipDisplayTests bool,
) *Result {
	programF := deploytest.NewLanguageRuntimeF(program)
	hostF := deploytest.NewPluginHostF(nil, nil, programF, b.loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{T: b.t, HostF: hostF, SkipDisplayTests: skipDisplayTests},
	}

	// Run an update for initial state.
	var err error
	snap, err := TestOp(engine.Update).Run(
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
