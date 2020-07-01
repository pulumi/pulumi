// Copyright 2016-2020, Pulumi Corporation.
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

package cli

import (
	"context"
	"fmt"
	"regexp"
	"strconv"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/v2/backend"
	"github.com/pulumi/pulumi/pkg/v2/backend/display"
	"github.com/pulumi/pulumi/pkg/v2/engine"
	"github.com/pulumi/pulumi/pkg/v2/operations"
	"github.com/pulumi/pulumi/pkg/v2/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v2/secrets"
	"github.com/pulumi/pulumi/pkg/v2/util/cancel"
	"github.com/pulumi/pulumi/sdk/v2/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v2/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v2/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v2/go/common/workspace"
)

// CancellationScope provides a scoped source of cancellation and termination requests.
type CancellationScope interface {
	// Context returns the cancellation context used to observe cancellation and termination requests for this scope.
	Context() *cancel.Context
	// Close closes the cancellation scope.
	Close()
}

// CancellationScopeSource provides a source for cancellation scopes.
type CancellationScopeSource interface {
	// NewScope creates a new cancellation scope.
	NewScope(events chan<- engine.Event, isPreview bool) CancellationScope
}

// UpdateMetadata describes optional metadata about an update.
type UpdateMetadata struct {
	// Message is an optional message associated with the update.
	Message string `json:"message"`
	// Environment contains optional data from the deploying environment. e.g. the current
	// source code control commit information.
	Environment map[string]string `json:"environment"`
}

// UpdateOperation is a complete stack update operation (preview, update, import, refresh, or destroy).
type UpdateOperation struct {
	Proj               *workspace.Project
	Root               string
	Imports            []deploy.Import
	M                  *UpdateMetadata
	Opts               UpdateOptions
	SecretsManager     secrets.Manager
	StackConfiguration StackConfiguration
	Scopes             CancellationScopeSource
}

// QueryOperation configures a query operation.
type QueryOperation struct {
	Proj               *workspace.Project
	Root               string
	Opts               UpdateOptions
	SecretsManager     secrets.Manager
	StackConfiguration StackConfiguration
	Scopes             CancellationScopeSource
}

// StackConfiguration holds the configuration for a stack and it's associated decrypter.
type StackConfiguration struct {
	Config    config.Map
	Decrypter config.Decrypter
}

// UpdateOptions is the full set of update options, including backend and engine options.
type UpdateOptions struct {
	// Engine contains all of the engine-specific options.
	Engine engine.UpdateOptions
	// Display contains all of the backend display options.
	Display display.Options

	// AutoApprove, when true, will automatically approve previews.
	AutoApprove bool
	// SkipPreview, when true, causes the preview step to be skipped.
	SkipPreview bool
}

// QueryOptions configures a query to operate against a backend and the engine.
type QueryOptions struct {
	// Engine contains all of the engine-specific options.
	Engine engine.UpdateOptions
	// Display contains all of the backend display options.
	Display display.Options
}

type Backend struct {
	d              diag.Sink
	client         backend.Client
	currentProject *workspace.Project
}

// NewBackend creates a new CLI backend using the given client.
func NewBackend(d diag.Sink, client backend.Client) (*Backend, error) {
	// When stringifying backend references, we take the current project (if present) into account.
	currentProject, err := workspace.DetectProject()
	if err != nil {
		currentProject = nil
	}

	return &Backend{
		d:              d,
		client:         client,
		currentProject: currentProject,
	}, nil
}

// Name returns a friendly name for this backend.
func (b *Backend) Name() string {
	return b.client.Name()
}

// URL returns a URL at which information about this backend may be seen.
func (b *Backend) URL() string {
	return b.client.URL()
}

// Client returns the client instance that implements the lower-level operations required by the backend.
func (b *Backend) Client() backend.Client {
	return b.client
}

// Returns the identity of the current user for the backend.
func (b *Backend) CurrentUser() (string, error) {
	return b.client.User(context.Background())
}

// CurrentStack reads the current stack and returns an instance connected to its backend provider.
func (b *Backend) CurrentStack(ctx context.Context) (*Stack, error) {
	w, err := workspace.New()
	if err != nil {
		return nil, err
	}

	stackName := w.Settings().Stack
	if stackName == "" {
		return nil, nil
	}

	ref, err := b.ParseStackIdentifier(stackName)
	if err != nil {
		return nil, err
	}

	return b.GetStack(ctx, ref)
}

// SetCurrentStack changes the current stack to the given stack identifier.
func (b *Backend) SetCurrentStack(id backend.StackIdentifier) error {
	// Switch the current workspace to that stack.
	w, err := workspace.New()
	if err != nil {
		return err
	}

	w.Settings().Stack = id.String()
	return w.Save()
}

func (b *Backend) policyClient() (backend.PolicyClient, error) {
	policyClient, ok := b.client.(backend.PolicyClient)
	if !ok {
		return nil, fmt.Errorf("the selected backend does not support policy packs")
	}
	return policyClient, nil
}

// ParsePolicyPackIdentifier parses a policy pack identifier.
func (b *Backend) ParsePolicyPackIdentifier(s string) (backend.PolicyPackIdentifier, error) {
	currentUser, err := b.CurrentUser()
	if err != nil {
		currentUser = ""
	}
	return backend.ParsePolicyPackIdentifier(s, currentUser, b.client.URL())
}

// GetPolicyPack returns a PolicyPack object tied to this backend, or nil if it cannot be found.
func (b *Backend) GetPolicyPack(ctx context.Context, policyPack string) (*PolicyPack, error) {
	policyClient, err := b.policyClient()
	if err != nil {
		return nil, err
	}

	id, err := b.ParsePolicyPackIdentifier(policyPack)
	if err != nil {
		return nil, err
	}

	return &PolicyPack{
		id: id,
		b:  b,
		cl: policyClient,
	}, nil
}

// ListPolicyGroups returns all Policy Groups for an organization in this backend or an error if it cannot be found.
func (b *Backend) ListPolicyGroups(ctx context.Context, orgName string) (apitype.ListPolicyGroupsResponse, error) {
	policyClient, err := b.policyClient()
	if err != nil {
		return apitype.ListPolicyGroupsResponse{}, err
	}
	return policyClient.ListPolicyGroups(ctx, orgName)
}

// ListPolicyPacks returns all Policy Packs for an organization in this backend, or an error if it cannot be found.
func (b *Backend) ListPolicyPacks(ctx context.Context, orgName string) (apitype.ListPolicyPacksResponse, error) {
	policyClient, err := b.policyClient()
	if err != nil {
		return apitype.ListPolicyPacksResponse{}, err
	}
	return policyClient.ListPolicyPacks(ctx, orgName)
}

// SupportsOrganizations tells whether a user can belong to multiple organizations in this backend.
func (b *Backend) SupportsOrganizations() bool {
	return true
}

// ParseStackIdentifier parses a stack identifier in the context of the current user and project.
func (b *Backend) ParseStackIdentifier(s string) (backend.StackIdentifier, error) {
	return backend.ParseStackIdentifierWithClient(context.Background(), s, b.client)
}

// ValidateStackName verifies that the string is a legal identifier for a (potentially qualified) stack.
func (b *Backend) ValidateStackName(s string) error {
	id, err := backend.ParseStackIdentifier(s, "", "")
	if err != nil {
		return err
	}

	// The Pulumi Service enforces specific naming restrictions for organizations,
	// projects, and stacks. Though ignore any values that need to be inferred later.
	if id.Owner != "" {
		if err := validateOwnerName(id.Owner); err != nil {
			return err
		}
	}

	if id.Project != "" {
		if err := validateProjectName(id.Project); err != nil {
			return err
		}
	}

	return validateStackName(id.Stack)
}

// StackFriendlyName returns the short form for a stack identifier using the current project and user context.
func (b *Backend) StackFriendlyName(id backend.StackIdentifier) string {
	currentUser, err := b.CurrentUser()
	if err != nil {
		currentUser = ""
	}
	return id.FriendlyName(currentUser, string(b.currentProject.Name))
}

// StackConsoleURL returns the Pulumi Console URL for the given stack identifier, if any. Callers should consider an
// empty URL and a nil error to indicate that the client does not support the Pulum Console.
func (b *Backend) StackConsoleURL(id backend.StackIdentifier) (string, error) {
	return b.client.StackConsoleURL(id)
}

// Name validation rules.
var (
	stackOwnerRegexp          = regexp.MustCompile("^[a-zA-Z0-9][a-zA-Z0-9-_]{1,38}[a-zA-Z0-9]$")
	stackNameAndProjectRegexp = regexp.MustCompile("^[A-Za-z0-9_.-]{1,100}$")
)

// validateOwnerName checks if a stack owner name is valid. An "owner" is simply the namespace
// a stack may exist within, which for the Pulumi Service is the user account or organization.
func validateOwnerName(s string) error {
	if !stackOwnerRegexp.MatchString(s) {
		return errors.New("invalid stack owner")
	}
	return nil
}

// validateStackName checks if a stack name is valid, returning a user-suitable error if needed.
func validateStackName(s string) error {
	if len(s) > 100 {
		return errors.New("stack names must be less than 100 characters")
	}
	if !stackNameAndProjectRegexp.MatchString(s) {
		return errors.New("stack names may only contain alphanumeric, hyphens, underscores, and periods")
	}
	return nil
}

// validateProjectName checks if a project name is valid, returning a user-suitable error if needed.
//
// NOTE: Be careful when requiring a project name be valid. The Pulumi.yaml file may contain
// an invalid project name like "r@bid^W0MBAT!!", but we try to err on the side of flexibility by
// implicitly "cleaning" the project name before we send it to the Pulumi Service. So when we go
// to make HTTP requests, we use a more palitable name like "r_bid_W0MBAT__".
//
// The projects canonical name will be the sanitized "r_bid_W0MBAT__" form, but we do not require the
// Pulumi.yaml file be updated.
//
// So we should only call validateProject name when creating _new_ stacks or creating _new_ projects.
// We should not require that project names be valid when reading what is in the current workspace.
func validateProjectName(s string) error {
	if len(s) > 100 {
		return errors.New("project names must be less than 100 characters")
	}
	if !stackNameAndProjectRegexp.MatchString(s) {
		return errors.New("project names may only contain alphanumeric, hyphens, underscores, and periods")
	}
	return nil
}

// Logout logs out of the backend.
func (b *Backend) Logout() error {
	return workspace.DeleteAccount(b.URL())
}

// DoesProjectExist returns true if a project with the given name exists in this backend, or false otherwise.
func (b *Backend) DoesProjectExist(ctx context.Context, projectName string) (bool, error) {
	owner, err := b.client.User(ctx)
	if err != nil {
		return false, err
	}

	return b.client.DoesProjectExist(ctx, owner, projectName)
}

// GetStack returns a stack object tied to this backend with the given identifier, or nil if it cannot be found.
func (b *Backend) GetStack(ctx context.Context, stackID backend.StackIdentifier) (*Stack, error) {
	stack, err := b.client.GetStack(ctx, stackID)
	if err != nil {
		if err == backend.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}

	return newStack(stack, b), nil
}

// CreateStack creates a new stack with the given name and options that are specific to the backend provider.
func (b *Backend) CreateStack(ctx context.Context, stackID backend.StackIdentifier) (*Stack, error) {
	tags, err := GetEnvironmentTagsForCurrentStack()
	if err != nil {
		return nil, errors.Wrap(err, "error determining initial tags")
	}

	// Confirm the stack identity matches the environment. e.g. stack init foo/bar/baz shouldn't work
	// if the project name in Pulumi.yaml is anything other than "bar".
	projNameTag, ok := tags[apitype.ProjectNameTag]
	if ok && stackID.Project != projNameTag {
		return nil, errors.Errorf("provided project name %q doesn't match Pulumi.yaml", stackID.Project)
	}

	apistack, err := b.client.CreateStack(ctx, stackID, tags)
	if err != nil {
		return nil, err
	}

	stack := newStack(apistack, b)
	fmt.Printf("Created stack '%s'\n", stack.id)

	return stack, nil
}

// ListStacks returns a list of stack summaries for all known stacks in the target backend.
func (b *Backend) ListStacks(ctx context.Context, filter backend.ListStacksFilter) ([]apitype.StackSummary, error) {
	// Sanitize the project name as needed, so when communicating with the Pulumi Service we
	// always use the name the service expects. (So that a similar, but not technically valid
	// name may be put in Pulumi.yaml without causing problems.)
	if filter.Project != nil {
		cleanedProj := cleanProjectName(*filter.Project)
		filter.Project = &cleanedProj
	}

	return b.client.ListStacks(ctx, filter)
}

// RemoveStack removes a stack with the given name. If force is true, the stack will be removed even if it still
// contains resources. Otherwise, if the stack contains resources, a non-nil error is returned, and the first boolean
// return value will be set to true.
func (b *Backend) RemoveStack(ctx context.Context, stack *Stack, force bool) (bool, error) {
	return b.client.DeleteStack(ctx, stack.id, force)
}

// RenameStack renames the given stack to a new name, and then returns an updated stack reference that can be used to
// refer to the newly renamed stack.
func (b *Backend) RenameStack(ctx context.Context, stack *Stack, newID string) (backend.StackIdentifier, error) {
	parsedID, err := b.ParseStackIdentifier(newID)
	if err != nil {
		return backend.StackIdentifier{}, err
	}

	if stack.id.Owner != parsedID.Owner {
		errMsg := fmt.Sprintf(
			"New stack owner, %s, does not match existing owner, %s.\n\n",
			stack.id.Owner, parsedID.Owner)

		parsedID, err = backend.ParseStackIdentifier(newID, "", "")
		if err == nil && parsedID.Owner == "" {
			errMsg += fmt.Sprintf(
				"       Did you forget to include the owner name? If yes, rerun the command as follows:\n\n"+
					"           $ pulumi stack rename %s/%s\n\n",
				stack.id.Owner, parsedID.Stack)
		}

		if consoleURL, err := stack.ConsoleURL(); err != nil && consoleURL != "" {
			errMsg += "       You cannot transfer stack ownership via a rename. If you wish to transfer ownership\n" +
				"       of a stack to another organization, you can do so in the Pulumi Console by going to the\n" +
				"       \"Settings\" page of the stack and then clicking the \"Transfer Stack\" button:\n" +
				"\n" +
				"           " + consoleURL + "/settings/options"
		}

		return backend.StackIdentifier{}, errors.New(errMsg)
	}

	if err = b.client.RenameStack(ctx, stack.id, parsedID); err != nil {
		return backend.StackIdentifier{}, err
	}
	return parsedID, nil
}

// Preview shows what would be updated given the current workspace's contents.
func (b *Backend) Preview(ctx context.Context, stack *Stack,
	op UpdateOperation) (engine.ResourceChanges, result.Result) {

	// We can skip PreviewtThenPromptThenExecute, and just go straight to Execute.
	opts := applierOptions{
		DryRun:   true,
		ShowLink: true,
	}
	return b.apply(
		ctx, apitype.PreviewUpdate, stack, op, opts, nil /*events*/)
}

// Update updates the target stack with the current workspace's contents (config and code).
func (b *Backend) Update(ctx context.Context, stack *Stack,
	op UpdateOperation) (engine.ResourceChanges, result.Result) {
	return previewThenPromptThenExecute(ctx, apitype.UpdateUpdate, stack, op, b.apply)
}

// Import imports resources into a stack.
func (b *Backend) Import(ctx context.Context, stack *Stack,
	op UpdateOperation, imports []deploy.Import) (engine.ResourceChanges, result.Result) {
	op.Imports = imports
	return previewThenPromptThenExecute(ctx, apitype.ResourceImportUpdate, stack, op, b.apply)
}

// Refresh refreshes the stack's state from the cloud provider.
func (b *Backend) Refresh(ctx context.Context, stack *Stack,
	op UpdateOperation) (engine.ResourceChanges, result.Result) {
	return previewThenPromptThenExecute(ctx, apitype.RefreshUpdate, stack, op, b.apply)
}

// Destroy destroys all of this stack's resources.
func (b *Backend) Destroy(ctx context.Context, stack *Stack,
	op UpdateOperation) (engine.ResourceChanges, result.Result) {
	return previewThenPromptThenExecute(ctx, apitype.DestroyUpdate, stack, op, b.apply)
}

// Watch watches the project's working directory for changes and automatically updates the active stack.
func (b *Backend) Watch(ctx context.Context, stack *Stack,
	op UpdateOperation) result.Result {
	return Watch(ctx, b, stack, op, b.apply)
}

// Query against the resource outputs in a stack's state checkpoint.
func (b *Backend) Query(ctx context.Context, op QueryOperation) result.Result {
	return b.query(ctx, op, nil /*events*/)
}

func (b *Backend) startUpdate(
	ctx context.Context, action apitype.UpdateKind, stack *Stack,
	op *UpdateOperation, dryRun bool) (backend.Update, error) {

	metadata := apitype.UpdateMetadata{
		Message:     op.M.Message,
		Environment: op.M.Environment,
	}

	// Start the update. We use this opportunity to pass new tags to the service, to pick up any
	// metadata changes.
	tags, err := GetMergedStackTags(ctx, stack)
	if err != nil {
		return nil, errors.Wrap(err, "getting stack tags")
	}

	return b.client.StartUpdate(ctx, action, stack.id, op.Proj, op.StackConfiguration.Config, metadata, op.Opts.Engine,
		tags, dryRun)
}

// apply actually performs the provided type of update on a stack hosted in the Pulumi Cloud.
func (b *Backend) apply(
	ctx context.Context, kind apitype.UpdateKind, stack *Stack,
	op UpdateOperation, opts applierOptions,
	events chan<- engine.Event) (engine.ResourceChanges, result.Result) {

	actionLabel := actionLabel(kind, opts.DryRun)

	if !(op.Opts.Display.JSONDisplay || op.Opts.Display.Type == display.DisplayWatch) {
		// Print a banner so it's clear this is going to the client.
		fmt.Printf(op.Opts.Display.Color.Colorize(
			colors.SpecHeadline+"%s (%s):"+colors.Reset+"\n"), actionLabel, stack.FriendlyName())
	}

	// Create an update object to persist results.
	update, err := b.startUpdate(ctx, kind, stack, &op, opts.DryRun)
	if err != nil {
		return nil, result.FromError(err)
	}

	// Set up required policies.
	for _, policy := range update.RequiredPolicies() {
		op.Opts.Engine.RequiredPolicies = append(op.Opts.Engine.RequiredPolicies, &requiredPolicy{
			meta:    policy,
			orgName: stack.orgName,
			client:  stack.b.client,
		})
	}

	if !op.Opts.Display.SuppressPermaLink && opts.ShowLink && !op.Opts.Display.JSONDisplay {
		// Print a URL at the beginning of the update pointing to the Pulumi Service.
		if url := update.ProgressURL(); url != "" {
			fmt.Printf(op.Opts.Display.Color.Colorize(
				colors.SpecHeadline+"View Live: "+
					colors.Underline+colors.BrightBlue+"%s"+colors.Reset+"\n\n"), url)
		}

	}

	// Run the update.
	changes, res := b.runEngineAction(ctx, kind, stack.id, op, update, events, opts.DryRun)

	// Make sure to print a link to the stack's checkpoint before exiting.
	if !op.Opts.Display.SuppressPermaLink && opts.ShowLink && !op.Opts.Display.JSONDisplay {
		if url := update.PermalinkURL(); url != "" && url != update.ProgressURL() {
			fmt.Printf(op.Opts.Display.Color.Colorize(
				colors.SpecHeadline+"Permalink: "+
					colors.Underline+colors.BrightBlue+"%s"+colors.Reset+"\n"), url)
		}
	}

	return changes, res
}

// query executes a query program against the resource outputs of a stack hosted in the Pulumi
// Cloud.
func (b *Backend) query(ctx context.Context, op QueryOperation,
	callerEventsOpt chan<- engine.Event) result.Result {

	return RunQuery(ctx, b, op, callerEventsOpt, b.newQuery)
}

func (b *Backend) runEngineAction(
	ctx context.Context, kind apitype.UpdateKind, stackID backend.StackIdentifier,
	op UpdateOperation, update backend.Update, callerEventsOpt chan<- engine.Event,
	dryRun bool) (engine.ResourceChanges, result.Result) {

	u, err := b.newUpdate(ctx, stackID, op, update)
	if err != nil {
		return nil, result.FromError(err)
	}

	// displayEvents renders the event to the console and Pulumi service. The processor for the
	// will signal all events have been proceed when a value is written to the displayDone channel.
	displayEvents := make(chan engine.Event)
	displayDone := make(chan bool)
	go u.RecordAndDisplayEvents(
		actionLabel(kind, dryRun), kind, stackID, op,
		displayEvents, displayDone, op.Opts.Display, dryRun)

	// The engineEvents channel receives all events from the engine, which we then forward onto other
	// channels for actual processing. (displayEvents and callerEventsOpt.)
	engineEvents := make(chan engine.Event)
	eventsDone := make(chan bool)
	go func() {
		for e := range engineEvents {
			displayEvents <- e
			if callerEventsOpt != nil {
				callerEventsOpt <- e
			}
		}

		close(eventsDone)
	}()

	// The backend.SnapshotManager and backend.SnapshotPersister will keep track of any changes to
	// the Snapshot (checkpoint file) in the HTTP backend. We will reuse the snapshot's secrets manager when possible
	// to ensure that secrets are not re-encrypted on each update.
	sm := op.SecretsManager
	if secrets.AreCompatible(sm, u.GetTarget().Snapshot.SecretsManager) {
		sm = u.GetTarget().Snapshot.SecretsManager
	}
	snapshotManager := backend.NewSnapshotManager(ctx, u.update, sm, u.GetTarget().Snapshot)

	// Depending on the action, kick off the relevant engine activity.  Note that we don't immediately check and
	// return error conditions, because we will do so below after waiting for the display channels to close.
	cancellationScope := op.Scopes.NewScope(engineEvents, dryRun)
	engineCtx := &engine.Context{
		Cancel:          cancellationScope.Context(),
		Events:          engineEvents,
		SnapshotManager: snapshotManager,
		BackendClient:   backend.NewBackendClient(b.client),
	}
	if parentSpan := opentracing.SpanFromContext(ctx); parentSpan != nil {
		engineCtx.ParentSpan = parentSpan.Context()
	}

	var changes engine.ResourceChanges
	var res result.Result
	switch kind {
	case apitype.PreviewUpdate:
		changes, res = engine.Update(u, engineCtx, op.Opts.Engine, true)
	case apitype.UpdateUpdate:
		changes, res = engine.Update(u, engineCtx, op.Opts.Engine, dryRun)
	case apitype.RefreshUpdate:
		changes, res = engine.Refresh(u, engineCtx, op.Opts.Engine, dryRun)
	case apitype.DestroyUpdate:
		changes, res = engine.Destroy(u, engineCtx, op.Opts.Engine, dryRun)
	default:
		contract.Failf("Unrecognized update kind: %s", kind)
	}

	// Wait for dependent channels to finish processing engineEvents before closing.
	<-displayDone
	cancellationScope.Close() // Don't take any cancellations anymore, we're shutting down.
	close(engineEvents)
	contract.IgnoreClose(snapshotManager)

	// Make sure that the goroutine writing to displayEvents and callerEventsOpt
	// has exited before proceeding
	<-eventsDone
	close(displayEvents)

	// Mark the update as complete.
	status := apitype.UpdateStatusSucceeded
	if res != nil {
		status = apitype.UpdateStatusFailed
	}
	completeErr := u.Complete(status)
	if completeErr != nil {
		res = result.Merge(res, result.FromError(errors.Wrap(completeErr, "failed to complete update")))
	}

	return changes, res
}

// CancelCurrentUpdate cancels the currently running update for the given stack, if any.
func (b *Backend) CancelCurrentUpdate(ctx context.Context, stackID backend.StackIdentifier) error {
	stack, err := b.client.GetStack(ctx, stackID)
	if err != nil {
		return err
	}

	if stack.ActiveUpdate == "" {
		return errors.Errorf("stack %v has never been updated", stackID)
	}

	return b.client.CancelCurrentUpdate(ctx, stackID)
}

// GetHistory returns all updates for the stack. The returned UpdateInfo slice will be in descending order (newest
// first).
func (b *Backend) GetHistory(ctx context.Context, stackID backend.StackIdentifier) ([]backend.UpdateInfo, error) {
	updates, err := b.client.GetStackHistory(ctx, stackID)
	if err != nil {
		return nil, err
	}

	// Convert apitype.UpdateInfo objects to the backend type.
	var beUpdates []backend.UpdateInfo
	for _, update := range updates {
		// Convert types from the apitype package into their internal counterparts.
		cfg, err := convertConfig(update.Config)
		if err != nil {
			return nil, errors.Wrap(err, "converting configuration")
		}

		beUpdates = append(beUpdates, backend.UpdateInfo{
			Kind:            update.Kind,
			Message:         update.Message,
			Environment:     update.Environment,
			Config:          cfg,
			Result:          backend.UpdateResult(update.Result),
			StartTime:       update.StartTime,
			EndTime:         update.EndTime,
			ResourceChanges: convertResourceChanges(update.ResourceChanges),
		})
	}

	return beUpdates, nil
}

// Get the configuration from the most recent deployment of the stack.
func (b *Backend) GetLatestConfiguration(ctx context.Context, stack *Stack) (config.Map, error) {
	return b.client.GetLatestStackConfig(ctx, stack.id)
}

// convertResourceChanges converts the apitype version of engine.ResourceChanges into the internal version.
func convertResourceChanges(changes map[apitype.OpType]int) engine.ResourceChanges {
	b := make(engine.ResourceChanges)
	for k, v := range changes {
		b[deploy.StepOp(k)] = v
	}
	return b
}

// convertResourceChanges converts the apitype version of config.Map into the internal version.
func convertConfig(apiConfig map[string]apitype.ConfigValue) (config.Map, error) {
	c := make(config.Map)
	for rawK, rawV := range apiConfig {
		k, err := config.ParseKey(rawK)
		if err != nil {
			return nil, err
		}
		if rawV.Object {
			if rawV.Secret {
				c[k] = config.NewSecureObjectValue(rawV.String)
			} else {
				c[k] = config.NewObjectValue(rawV.String)
			}
		} else {
			if rawV.Secret {
				c[k] = config.NewSecureValue(rawV.String)
			} else {
				c[k] = config.NewValue(rawV.String)
			}
		}
	}
	return c, nil
}

// GetLogs fetches a list of log entries for the given stack, with optional filtering/querying.
func (b *Backend) GetLogs(ctx context.Context, stack *Stack, cfg StackConfiguration,
	logQuery operations.LogQuery) ([]operations.LogEntry, error) {

	target, targetErr := b.getTarget(ctx, stack.id, cfg.Config, cfg.Decrypter)
	if targetErr != nil {
		return nil, targetErr
	}
	return backend.GetLogsForTarget(target, logQuery)
}

// ExportDeployment exports the deployment for the given stack as an opaque JSON message.
func (b *Backend) ExportDeployment(ctx context.Context,
	stack *Stack) (*apitype.UntypedDeployment, error) {
	return b.exportDeployment(ctx, stack.id, nil /* latest */)
}

// ExportDeploymentForVersion exports a specific deployment from the history of a stack. The meaning of version is
// client-specific. For the Pulumi client, it is a simple numeric version (the first update being version "1", the
// second "2", and so on), though this might change in the future to use some other type of identifier or commitish.
func (b *Backend) ExportDeploymentForVersion(
	ctx context.Context, stack *Stack, version string) (*apitype.UntypedDeployment, error) {
	// The Pulumi Console defines versions as a positive integer. Parse the provided version string and
	// ensure it is valid.
	//
	// The first stack update version is 1, and monotonically increasing from there.
	versionNumber, err := strconv.Atoi(version)
	if err != nil || versionNumber <= 0 {
		return nil, errors.Errorf("%q is not a valid stack version. It should be a positive integer.", version)
	}

	return b.exportDeployment(ctx, stack.id, &versionNumber)
}

// exportDeployment exports the checkpoint file for a stack, optionally getting a previous version.
func (b *Backend) exportDeployment(ctx context.Context, stackID backend.StackIdentifier,
	version *int) (*apitype.UntypedDeployment, error) {

	deployment, err := b.client.ExportStackDeployment(ctx, stackID, version)
	if err != nil {
		return nil, err
	}

	return &deployment, nil
}

// ImportDeployment imports the given deployment into the indicated stack.
func (b *Backend) ImportDeployment(ctx context.Context, stack *Stack, deployment *apitype.UntypedDeployment) error {
	return b.client.ImportStackDeployment(ctx, stack.id, deployment)
}

// GetStackTags fetches the stack's existing tags.
func (b *Backend) GetStackTags(ctx context.Context, stack *Stack) (map[apitype.StackTagName]string, error) {
	return stack.tags, nil
}

// UpdateStackTags updates the stacks's tags, replacing all existing tags.
func (b *Backend) UpdateStackTags(ctx context.Context, stack *Stack, tags map[apitype.StackTagName]string) error {
	return b.client.UpdateStackTags(ctx, stack.id, tags)
}

var projectNameCleanRegexp = regexp.MustCompile("[^a-zA-Z0-9-_.]")

// cleanProjectName replaces undesirable characters in project names with hyphens. At some point, these restrictions
// will be further enforced by the service, but for now we need to ensure that if we are making a rest call, we
// do this cleaning on our end.
func cleanProjectName(projectName string) string {
	return projectNameCleanRegexp.ReplaceAllString(projectName, "-")
}
