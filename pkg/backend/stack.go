// Copyright 2016-2018, Pulumi Corporation.
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

package backend

import (
	"context"
	"fmt"
	"strings"
	"time"

	opentracing "github.com/opentracing/opentracing-go"
	backendDisplay "github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/operations"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/util/nosleep"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/gitutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type StackConfigLocation struct {
	// IsRemote indicates if the stack's configuration is stored remotely instead of in a local file.
	IsRemote bool
	// EscEnv is the optional name of an ESC Environment being used to store the stack's configuration.
	EscEnv *string
}

// Stack is used to manage stacks of resources against a pluggable backend.
type Stack interface {
	// Ref returns this stack's identity.
	Ref() StackReference
	// ConfigLocation indicates if the backend has configuration stored independent of the local file stack config.
	ConfigLocation() StackConfigLocation
	// LoadRemoteConfig the stack's configuration remotely from the backend.
	LoadRemoteConfig(ctx context.Context, project *workspace.Project) (*workspace.ProjectStack, error)
	// SaveRemoteConfig the stack's configuration remotely to the backend.
	SaveRemoteConfig(ctx context.Context, projectStack *workspace.ProjectStack) error
	// Snapshot returns the latest deployment snapshot.
	Snapshot(ctx context.Context, secretsProvider secrets.Provider) (*deploy.Snapshot, error)
	// Backend returns the backend this stack belongs to.
	Backend() Backend
	// Tags return the stack's existing tags.
	Tags() map[apitype.StackTagName]string

	// DefaultSecretManager returns the default secrets manager to use for this stack. This may be more specific than
	// Backend.DefaultSecretManager.
	DefaultSecretManager(info *workspace.ProjectStack) (secrets.Manager, error)
}

// RemoveStack returns the stack, or returns an error if it cannot.
func RemoveStack(ctx context.Context, s Stack, force bool) (bool, error) {
	return s.Backend().RemoveStack(ctx, s, force)
}

// RenameStack renames the stack, or returns an error if it cannot.
func RenameStack(ctx context.Context, s Stack, newName tokens.QName) (StackReference, error) {
	return s.Backend().RenameStack(ctx, s, newName)
}

// PreviewStack previews changes to this stack.
func PreviewStack(
	ctx context.Context,
	s Stack,
	op UpdateOperation,
	events chan<- engine.Event,
) (*deploy.Plan, display.ResourceChanges, error) {
	return s.Backend().Preview(ctx, s, op, events)
}

// UpdateStack updates the target stack with the current workspace's contents (config and code).
func UpdateStack(
	ctx context.Context,
	s Stack,
	op UpdateOperation,
	events chan<- engine.Event,
) (display.ResourceChanges, error) {
	return s.Backend().Update(ctx, s, op, events)
}

// ApplyStack applies a given operation to the passed stack.
func ApplyStack(
	ctx context.Context,
	kind apitype.UpdateKind,
	stack Stack,
	op UpdateOperation,
	opts ApplierOptions,
	events chan<- engine.Event,
) (*deploy.Plan, display.ResourceChanges, error) {
	resetKeepRunning := nosleep.KeepRunning()
	defer resetKeepRunning()

	b := stack.Backend()

	err := b.CheckApply(ctx, stack)
	if err != nil {
		return nil, nil, err
	}

	stackRef := stack.Ref()
	actionLabel := ActionLabel(kind, opts.DryRun)

	if !op.Opts.Display.JSONDisplay && op.Opts.Display.Type != backendDisplay.DisplayWatch {
		fmt.Printf(op.Opts.Display.Color.Colorize(
			colors.SpecHeadline+"%s (%s)"+colors.Reset+"\n\n"), actionLabel, stackRef)
	}

	// Begin the apply on the backend.
	app, target, appEvents, appEventsDone, err := b.BeginApply(ctx, kind, stack, &op, opts)
	if err != nil {
		return nil, nil, err
	}

	update := engine.UpdateInfo{
		Root:    op.Root,
		Project: op.Proj,
		Target:  target,
	}

	// Create a separate event channel to power the display. We'll pipe all events we receive from the engine to this
	// channel in order to render things as they happen.
	displayEvents := make(chan engine.Event)
	displayDone := make(chan bool)
	go backendDisplay.ShowEvents(
		strings.ToLower(actionLabel), kind, stackRef.Name(), op.Proj.Name, app.Permalink(),
		displayEvents, displayDone, op.Opts.Display, opts.DryRun)

	// Create the channel on which we'll receive events from the engine and kick off a Goroutine to forward them on to
	// appropriate listeners:
	//   - the display, as initialized above
	//   - the backend, if BeginApply returned an appEvents channel
	//	 - the caller, if the caller provided an events channel
	engineEvents := make(chan engine.Event)

	scope := op.Scopes.NewScope(engineEvents, opts.DryRun)
	eventsDone := make(chan bool)
	go func() {
		for e := range engineEvents {
			displayEvents <- e

			if appEvents != nil {
				appEvents <- e
			}

			if events != nil {
				events <- e
			}
		}

		// When the engine closes the channel, notify display and backend-specific listeners that we're done. Presently, we
		// don't have a use case for notifying the caller since the current contract is that no more events will be sent
		// once this function returns.
		close(displayEvents)
		if appEvents != nil {
			close(appEvents)
		}

		close(eventsDone)
	}()

	// Create the management machinery. We only need a snapshot manager if we're doing an update.
	var manager *SnapshotManager
	if kind != apitype.PreviewUpdate && !opts.DryRun {
		persister, err := app.CreateSnapshotPersister()
		if err != nil {
			return nil, nil, fmt.Errorf("getting snapshot persister: %w", err)
		}

		manager = NewSnapshotManager(persister, op.SecretsManager, update.Target.Snapshot)
	}

	backendClient, err := app.CreateBackendClient()
	if err != nil {
		return nil, nil, fmt.Errorf("creating backend client: %w", err)
	}

	engineCtx := &engine.Context{
		Cancel:          scope.Context(),
		Events:          engineEvents,
		SnapshotManager: manager,
		BackendClient:   backendClient,
	}
	if parentSpan := opentracing.SpanFromContext(ctx); parentSpan != nil {
		engineCtx.ParentSpan = parentSpan.Context()
	}

	// Invoke the engine to actually perform the operation.
	start := time.Now().Unix()
	var plan *deploy.Plan
	var changes display.ResourceChanges
	var updateErr error
	switch kind {
	case apitype.PreviewUpdate:
		plan, changes, updateErr = engine.Update(update, engineCtx, op.Opts.Engine, true)
	case apitype.UpdateUpdate:
		_, changes, updateErr = engine.Update(update, engineCtx, op.Opts.Engine, opts.DryRun)
	case apitype.ResourceImportUpdate:
		_, changes, updateErr = engine.Import(update, engineCtx, op.Opts.Engine, op.Imports, opts.DryRun)
	case apitype.RefreshUpdate:
		if op.Opts.Engine.RefreshProgram {
			_, changes, updateErr = engine.RefreshV2(update, engineCtx, op.Opts.Engine, opts.DryRun)
		} else {
			_, changes, updateErr = engine.Refresh(update, engineCtx, op.Opts.Engine, opts.DryRun)
		}
	case apitype.DestroyUpdate:
		if op.Opts.Engine.DestroyProgram {
			_, changes, updateErr = engine.DestroyV2(update, engineCtx, op.Opts.Engine, opts.DryRun)
		} else {
			_, changes, updateErr = engine.Destroy(update, engineCtx, op.Opts.Engine, opts.DryRun)
		}
	case apitype.StackImportUpdate, apitype.RenameUpdate:
		contract.Failf("unexpected %s event", kind)
	default:
		contract.Failf("Unrecognized update kind: %s", kind)
	}
	end := time.Now().Unix()

	// Wait for the display to finish showing all the events.
	<-displayDone

	scope.Close() // Don't take any cancellations anymore, we're shutting down.
	close(engineEvents)
	if manager != nil {
		err = manager.Close()
		// If the snapshot manager failed to close, we should return that error. Even though all the parts of the operation
		// have potentially succeeded, a snapshotting failure is likely to rear its head on the next operation/invocation
		// (e.g. an invalid snapshot that fails integrity checks, or a failure to write that means the snapshot is
		// incomplete). Reporting now should make debugging and reporting easier.
		if err != nil {
			return plan, changes, fmt.Errorf("writing snapshot: %w", err)
		}
	}

	// We have already waited for the display to finish. Before continuing further, wait also for any backend-specific
	// and caller event processors to finish.
	<-eventsDone
	if appEventsDone != nil {
		<-appEventsDone
	}

	// Perform any backend-specific completion steps, such as writing stack history or metadata.
	err = app.End(start, end, plan, changes, updateErr)
	if err != nil {
		return plan, changes, err
	}

	return plan, changes, nil
}

// ImportStack updates the target stack with the current workspace's contents (config and code).
func ImportStack(ctx context.Context, s Stack, op UpdateOperation,
	imports []deploy.Import,
) (display.ResourceChanges, error) {
	return s.Backend().Import(ctx, s, op, imports)
}

// RefreshStack refresh's the stack's state from the cloud provider.
func RefreshStack(ctx context.Context, s Stack, op UpdateOperation) (display.ResourceChanges, error) {
	return s.Backend().Refresh(ctx, s, op)
}

// DestroyStack destroys all of this stack's resources.
func DestroyStack(ctx context.Context, s Stack, op UpdateOperation) (display.ResourceChanges, error) {
	return s.Backend().Destroy(ctx, s, op)
}

// WatchStack watches the projects working directory for changes and automatically updates the
// active stack.
func WatchStack(ctx context.Context, s Stack, op UpdateOperation, paths []string) error {
	return s.Backend().Watch(ctx, s, op, paths)
}

// GetLatestConfiguration returns the configuration for the most recent deployment of the stack.
func GetLatestConfiguration(ctx context.Context, s Stack) (config.Map, error) {
	return s.Backend().GetLatestConfiguration(ctx, s)
}

// GetStackLogs fetches a list of log entries for the current stack in the current backend.
func GetStackLogs(ctx context.Context, secretsProvider secrets.Provider, s Stack, cfg StackConfiguration,
	query operations.LogQuery,
) ([]operations.LogEntry, error) {
	return s.Backend().GetLogs(ctx, secretsProvider, s, cfg, query)
}

// ExportStackDeployment exports the given stack's deployment as an opaque JSON message.
func ExportStackDeployment(
	ctx context.Context,
	s Stack,
) (*apitype.UntypedDeployment, error) {
	return s.Backend().ExportDeployment(ctx, s)
}

// ImportStackDeployment imports the given deployment into the indicated stack.
func ImportStackDeployment(ctx context.Context, s Stack, deployment *apitype.UntypedDeployment) error {
	return s.Backend().ImportDeployment(ctx, s, deployment)
}

// UpdateStackTags updates the stacks's tags, replacing all existing tags.
func UpdateStackTags(ctx context.Context, s Stack, tags map[apitype.StackTagName]string) error {
	return s.Backend().UpdateStackTags(ctx, s, tags)
}

// GetMergedStackTags returns the stack's existing tags merged with fresh tags from the environment
// and Pulumi.yaml file.
func GetMergedStackTags(ctx context.Context, s Stack,
	root string, project *workspace.Project, cfg config.Map,
) (map[apitype.StackTagName]string, error) {
	// Get the stack's existing tags.
	tags := s.Tags()
	if tags == nil {
		tags = make(map[apitype.StackTagName]string)
	}

	// Get latest environment tags for the current stack.
	envTags, err := GetEnvironmentTagsForCurrentStack(root, project, cfg)
	if err != nil {
		return nil, err
	}

	// Add each new environment tag to the existing tags, overwriting existing tags with the
	// latest values.
	for k, v := range envTags {
		tags[k] = v
	}

	return tags, nil
}

// GetEnvironmentTagsForCurrentStack returns the set of tags for the "current" stack, based on the environment
// and Pulumi.yaml file.
func GetEnvironmentTagsForCurrentStack(root string,
	project *workspace.Project, cfg config.Map,
) (map[apitype.StackTagName]string, error) {
	tags := make(map[apitype.StackTagName]string)

	// Tags based on Pulumi.yaml.
	if project != nil {
		tags[apitype.ProjectNameTag] = project.Name.String()
		tags[apitype.ProjectRuntimeTag] = project.Runtime.Name()
		if project.Description != nil {
			tags[apitype.ProjectDescriptionTag] = *project.Description
		}
	}

	// Grab any `pulumi:tag` config values and use those to update the stack's tags.
	configTags, has, err := cfg.Get(config.MustParseKey(apitype.PulumiTagsConfigKey), false)
	contract.AssertNoErrorf(err, "Config.Get(\"%s\") failed unexpectedly", apitype.PulumiTagsConfigKey)
	if has {
		configTagInterface, err := configTags.ToObject()
		if err != nil {
			return nil, fmt.Errorf("%s must be an object of strings", apitype.PulumiTagsConfigKey)
		}
		configTagObject, ok := configTagInterface.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("%s must be an object of strings", apitype.PulumiTagsConfigKey)
		}

		for name, value := range configTagObject {
			stringValue, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("%s[%s] must be a string", apitype.PulumiTagsConfigKey, name)
			}

			tags[name] = stringValue
		}
	}

	// Add the git metadata to the tags, ignoring any errors that come from it.
	if root != "" {
		ignoredErr := addGitMetadataToStackTags(tags, root)
		contract.IgnoreError(ignoredErr)
	}

	return tags, nil
}

// addGitMetadataToStackTags fetches the git repository from the directory, and attempts to detect
// and add any relevant git metadata as stack tags.
func addGitMetadataToStackTags(tags map[apitype.StackTagName]string, projPath string) error {
	repo, err := gitutil.GetGitRepository(projPath)
	if repo == nil {
		return fmt.Errorf("no git repository found from %v", projPath)
	}
	if err != nil {
		return err
	}

	remoteURL, err := gitutil.GetGitRemoteURL(repo, "origin")
	if err != nil {
		return err
	}
	if remoteURL == "" {
		return nil
	}

	if vcsInfo, err := gitutil.TryGetVCSInfo(remoteURL); err == nil {
		tags[apitype.VCSOwnerNameTag] = vcsInfo.Owner
		tags[apitype.VCSRepositoryNameTag] = vcsInfo.Repo
		tags[apitype.VCSRepositoryKindTag] = vcsInfo.Kind
	} else {
		return fmt.Errorf("detecting VCS info for stack tags for remote %v: %w", remoteURL, err)
	}

	return nil
}
