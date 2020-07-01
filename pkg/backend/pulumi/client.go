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

package pulumi

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pulumi/pulumi/pkg/v2/backend"
	"github.com/pulumi/pulumi/pkg/v2/backend/filestate"
	"github.com/pulumi/pulumi/pkg/v2/backend/pulumi/client"
	"github.com/pulumi/pulumi/pkg/v2/engine"
	"github.com/pulumi/pulumi/pkg/v2/secrets/service"
	"github.com/pulumi/pulumi/sdk/v2/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v2/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/retry"
	"github.com/pulumi/pulumi/sdk/v2/go/common/workspace"
)

const (
	// defaultAPIEnvVar can be set to override the default cloud chosen, if `--cloud` is not present.
	defaultURLEnvVar = "PULUMI_API"
	// AccessTokenEnvVar is the environment variable used to bypass a prompt on login.
	AccessTokenEnvVar = "PULUMI_ACCESS_TOKEN"
)

// DefaultURL returns the default cloud URL.  This may be overridden using the PULUMI_API environment
// variable.  If no override is found, and we are authenticated with a cloud, choose that.  Otherwise,
// we will default to the https://api.pulumi.com/ endpoint.
func DefaultURL() string {
	return ValueOrDefaultURL("")
}

// ValueOrDefaultURL returns the value if specified, or the default cloud URL otherwise.
func ValueOrDefaultURL(cloudURL string) string {
	// If we have a cloud URL, just return it.
	if cloudURL != "" {
		return cloudURL
	}

	// Otherwise, respect the PULUMI_API override.
	if cloudURL := os.Getenv(defaultURLEnvVar); cloudURL != "" {
		return cloudURL
	}

	// If that didn't work, see if we have a current cloud, and use that. Note we need to be careful
	// to ignore the local cloud.
	if creds, err := workspace.GetStoredCredentials(); err == nil {
		if creds.Current != "" && !filestate.IsFileStateBackendURL(creds.Current) {
			return creds.Current
		}
	}

	// If none of those led to a cloud URL, simply return the default.
	return PulumiCloudURL
}

var _ backend.Client = (*Client)(nil)

type Client struct {
	client *client.Client
}

type recordEventRequest struct {
	ctx   context.Context
	event apitype.EngineEvent
	err   chan<- error
}

type eventBatcher struct {
	updateID    client.UpdateIdentifier
	tokenSource *tokenSource
	client      *client.Client

	wg     sync.WaitGroup
	events chan<- recordEventRequest
}

type serviceUpdate struct {
	updateID         client.UpdateIdentifier
	requiredPolicies []apitype.RequiredPolicy
	tokenSource      *tokenSource
	client           *client.Client
	batcher          *eventBatcher
	preview          bool
	version          int
}

func clientStackID(stackID backend.StackIdentifier) client.StackIdentifier {
	return client.StackIdentifier{
		Owner:   stackID.Owner,
		Project: stackID.Project,
		Stack:   stackID.Stack,
	}
}

func NewClient(d diag.Sink, url string) (backend.Client, error) {
	url = ValueOrDefaultURL(url)
	account, err := workspace.GetAccount(url)
	if err != nil {
		return nil, fmt.Errorf("getting stored credentials: %w", err)
	}
	apiToken := account.AccessToken

	return &Client{
		client: client.NewClient(url, apiToken, d),
	}, nil
}

func (c *Client) APIClient() *client.Client {
	return c.client
}

func (c *Client) Name() string {
	if c.client.URL() == PulumiCloudURL {
		return "pulumi.com"
	}
	return c.client.URL()
}

func (c *Client) URL() string {
	user, err := c.User(context.Background())
	if err != nil {
		return cloudConsoleURL(c.client.URL())
	}
	return cloudConsoleURL(c.client.URL(), user)
}

func (c *Client) User(ctx context.Context) (string, error) {
	account, err := workspace.GetAccount(c.client.URL())
	if err != nil {
		return "", err
	}
	if account.Username != "" {
		logging.V(1).Infof("found username for access token")
		return account.Username, nil
	}
	logging.V(1).Infof("no username for access token")
	return c.client.GetPulumiAccountName(ctx)
}

func (c *Client) DefaultSecretsManager() string {
	return service.Type
}

func (c *Client) DoesProjectExist(ctx context.Context, owner, projectName string) (bool, error) {
	return c.client.DoesProjectExist(ctx, owner, projectName)
}

func (c *Client) StackConsoleURL(stackID backend.StackIdentifier) (string, error) {
	stackPath := path.Join(stackID.Owner, stackID.Project, stackID.Stack)
	url := cloudConsoleURL(c.client.URL(), stackPath)
	if url == "" {
		return "", fmt.Errorf("could not determine cloud console URL")
	}
	return url, nil
}

func (c *Client) ListStacks(ctx context.Context, filter backend.ListStacksFilter) ([]apitype.StackSummary, error) {
	return c.client.ListStacks(ctx, client.ListStacksFilter{
		Project:      filter.Project,
		Organization: filter.Organization,
		TagName:      filter.TagName,
		TagValue:     filter.TagValue,
	})
}

func (c *Client) GetStack(ctx context.Context, stackID backend.StackIdentifier) (apitype.Stack, error) {
	stack, err := c.client.GetStack(ctx, clientStackID(stackID))
	if errResp, ok := err.(*apitype.ErrorResponse); ok && errResp.Code == http.StatusNotFound {
		return apitype.Stack{}, backend.ErrNotFound
	}
	return stack, err
}

func (c *Client) CreateStack(ctx context.Context, stackID backend.StackIdentifier,
	tags map[string]string) (apitype.Stack, error) {

	s, err := c.client.CreateStack(ctx, clientStackID(stackID), tags)
	// Wire through well-known error types.
	if errResp, ok := err.(*apitype.ErrorResponse); ok && errResp.Code == http.StatusConflict {
		// A 409 error response is returned when per-stack organizations are over their limit,
		// so we need to look at the message to differentiate.
		if strings.Contains(errResp.Message, "already exists") {
			return apitype.Stack{}, &backend.StackAlreadyExistsError{StackName: stackID.String()}
		}
		if strings.Contains(errResp.Message, "you are using") {
			return apitype.Stack{}, &backend.OverStackLimitError{Message: errResp.Message}
		}
	}
	return s, err
}

func (c *Client) DeleteStack(ctx context.Context, stackID backend.StackIdentifier, force bool) (bool, error) {
	return c.client.DeleteStack(ctx, clientStackID(stackID), force)
}

func (c *Client) RenameStack(ctx context.Context, currentID, newID backend.StackIdentifier) error {
	return c.client.RenameStack(ctx, clientStackID(currentID), clientStackID(newID))
}

func (c *Client) UpdateStackTags(ctx context.Context, stack backend.StackIdentifier, tags map[string]string) error {
	return c.client.UpdateStackTags(ctx, clientStackID(stack), tags)
}

func (c *Client) GetStackHistory(ctx context.Context, stackID backend.StackIdentifier) ([]apitype.UpdateInfo, error) {
	return c.client.GetStackUpdates(ctx, clientStackID(stackID))
}

func (c *Client) GetLatestStackConfig(ctx context.Context, stackID backend.StackIdentifier) (config.Map, error) {
	cfg, err := c.client.GetLatestConfiguration(ctx, clientStackID(stackID))
	switch {
	case err == client.ErrNoPreviousDeployment:
		return nil, backend.ErrNoPreviousDeployment
	case err != nil:
		return nil, err
	default:
		return cfg, nil
	}
}

func (c *Client) ExportStackDeployment(ctx context.Context, stackID backend.StackIdentifier,
	version *int) (apitype.UntypedDeployment, error) {

	return c.client.ExportStackDeployment(ctx, clientStackID(stackID), version)
}

func (c *Client) ImportStackDeployment(ctx context.Context, stackID backend.StackIdentifier,
	deployment *apitype.UntypedDeployment) error {

	update, err := c.client.ImportStackDeployment(ctx, clientStackID(stackID), deployment)
	if err != nil {
		return err
	}

	// Wait for the import to complete.
	var continuationToken *string
	_, status, err := retry.Until(context.Background(), retry.Acceptor{
		Accept: func(_ int, _ time.Duration) (bool, interface{}, error) {
			r, err := c.client.GetUpdateEvents(ctx, update, continuationToken)
			if err != nil {
				return false, nil, err
			}
			continuationToken = r.ContinuationToken
			isComplete := r.Status == apitype.StatusFailed || r.Status == apitype.StatusSucceeded
			return isComplete, r.Status, nil
		},
	})
	if err != nil {
		return fmt.Errorf("waiting for import: %w", err)
	}
	if status.(apitype.UpdateStatus) != apitype.StatusSucceeded {
		return fmt.Errorf("import unsuccessful: status %v", status)
	}
	return nil
}

func (c *Client) StartUpdate(ctx context.Context, kind apitype.UpdateKind, stackID backend.StackIdentifier,
	proj *workspace.Project, cfg config.Map, metadata apitype.UpdateMetadata, opts engine.UpdateOptions,
	tags map[string]string, dryRun bool) (backend.Update, error) {

	// TODO[pulumi-service#3745]: Move required policies to the plugin-gathering routine when we have a dedicated
	// service API when for getting a list of the required policies to run.
	//
	// For now, this list is given to us when we start an update; yet, the list of analyzers to boot
	// is given to us by CLI flag, and passed to the step generator (which lazily instantiates the
	// plugins) via `op.Opts.Engine.Analyzers`. Since the "start update" API request is sent well
	// after this field is populated, we instead populate the `RequiredPlugins` field here.
	//
	// Once this API is implemented, we can safely move these lines to the plugin-gathering code,
	// which is much closer to being the "correct" place for this stuff.

	update, reqdPolicies, err := c.client.CreateUpdate(ctx, kind, clientStackID(stackID), proj, cfg, metadata, opts,
		dryRun)
	if err != nil {
		return nil, err
	}

	version, token, err := c.client.StartUpdate(ctx, update, tags)
	if err != nil {
		if err, ok := err.(*apitype.ErrorResponse); ok && err.Code == 409 {
			conflict := backend.ConflictingUpdateError{Err: err}
			return nil, conflict
		}
		return nil, err
	}
	// Any non-preview update will be considered part of the stack's update history.
	if kind != apitype.PreviewUpdate {
		logging.V(7).Infof("Stack %s being updated to version %d", stackID, version)
	}

	var tokenSource *tokenSource
	if token != "" {
		ts, err := newTokenSource(ctx, token, c.client, update, 5*time.Minute)
		if err != nil {
			return nil, err
		}
		tokenSource = ts
	}

	return &serviceUpdate{
		updateID:         update,
		requiredPolicies: reqdPolicies,
		tokenSource:      tokenSource,
		client:           c.client,
		batcher:          startEventBatcher(update, tokenSource, c.client),
		preview:          dryRun,
		version:          version,
	}, nil
}

func (c *Client) CancelCurrentUpdate(ctx context.Context, stackID backend.StackIdentifier) error {
	stack, err := c.client.GetStack(ctx, clientStackID(stackID))
	if err != nil {
		return err
	}

	if stack.ActiveUpdate == "" {
		return fmt.Errorf("stack %v has never been updated", stackID)
	}

	// Compute the update identifier and attempt to cancel the update.
	//
	// NOTE: the update kind is not relevant; the same endpoint will work for updates of all kinds.
	updateID := client.UpdateIdentifier{
		StackIdentifier: clientStackID(stackID),
		UpdateKind:      apitype.UpdateUpdate,
		UpdateID:        stack.ActiveUpdate,
	}
	return c.client.CancelUpdate(ctx, updateID)
}

func (u *serviceUpdate) ProgressURL() string {
	return u.PermalinkURL()
}

func (u *serviceUpdate) PermalinkURL() string {
	stackID := u.updateID.StackIdentifier
	stackPath := path.Join(stackID.Owner, stackID.Project, stackID.Stack)

	if !u.preview {
		return cloudConsoleURL(u.client.URL(), stackPath, "updates", strconv.Itoa(u.version))
	}
	return cloudConsoleURL(u.client.URL(), stackPath, "previews", u.updateID.UpdateID)
}

func (u *serviceUpdate) RequiredPolicies() []apitype.RequiredPolicy {
	return u.requiredPolicies
}

func (u *serviceUpdate) RecordEvent(ctx context.Context, event apitype.EngineEvent) error {
	return u.batcher.recordEvent(ctx, event)
}

func (u *serviceUpdate) PatchCheckpoint(ctx context.Context, deployment *apitype.DeploymentV3) error {
	token, err := u.tokenSource.GetToken()
	if err != nil {
		return err
	}
	return u.client.PatchUpdateCheckpoint(ctx, u.updateID, deployment, token)
}

func (u *serviceUpdate) Complete(ctx context.Context, status apitype.UpdateStatus) error {
	token, err := u.tokenSource.GetToken()
	if err != nil {
		return err
	}
	return u.client.CompleteUpdate(ctx, u.updateID, status, token)
}

type recordBatchRequest struct {
	ctx   context.Context
	batch apitype.EngineEventBatch
	err   chan<- error
}

func startEventBatcher(updateID client.UpdateIdentifier, tokenSource *tokenSource,
	client *client.Client) *eventBatcher {

	events := make(chan recordEventRequest)

	b := &eventBatcher{
		updateID:    updateID,
		tokenSource: tokenSource,
		client:      client,
		events:      events,
	}

	// A single update can emit hundreds, if not thousands, or tens of thousands of
	// engine events. We transmit engine events in large batches to reduce the overhead
	// associated with each HTTP request to the service. We also send multiple HTTP
	// requests concurrently, as to not block processing subsequent engine events.

	// Maximum number of concurrent requests to the Pulumi Service to persist
	// engine events.
	const maxConcurrentRequests = 3

	// As we identify batches of engine events to transmit, we put them into a channel.
	// This will allow us to issue HTTP requests concurrently, but also limit the maximum
	// number of requests in-flight at any one time.
	//
	// This channel isn't buffered, so adding a new batch of events to persist will block
	// until a go-routine is available to send the batch.
	batches := make(chan recordBatchRequest)

	// Start N different go-routines which will all pull from the batchesToTransmit channel
	// and persist those engine events until the channel is closed.
	for i := 0; i < maxConcurrentRequests; i++ {
		go b.recordBatchWorker(batches)
	}

	// Start the event batcher.
	go b.batchEvents(events, batches)

	return b
}

func (b *eventBatcher) recordEvent(ctx context.Context, event apitype.EngineEvent) error {
	if event.CancelEvent != nil {
		close(b.events)
		b.wg.Wait()
		return nil
	}

	err := make(chan error)
	b.events <- recordEventRequest{ctx: ctx, event: event, err: err}
	return <-err
}

func (b *eventBatcher) recordBatch(ctx context.Context, batch apitype.EngineEventBatch) error {
	token, err := b.tokenSource.GetToken()
	if err != nil {
		return err
	}
	return b.client.RecordEngineEvents(ctx, b.updateID, batch, token)
}

func (b *eventBatcher) recordBatchWorker(requests <-chan recordBatchRequest) {
	b.wg.Add(1)
	defer b.wg.Done()

	for req := range requests {
		err := b.recordBatch(req.ctx, req.batch)
		if req.err != nil {
			req.err <- err
			close(req.err)
		}
	}
}

func (b *eventBatcher) batchEvents(events <-chan recordEventRequest, batches chan<- recordBatchRequest) {
	// Maximum number of events to batch up before transmitting.
	const maxEventsToTransmit = 50

	// Maximum wait time before sending all batched events.
	const maxTransmissionDelay = 4 * time.Second

	// transmitBatch sends off the current batch of engine events (eventIdx, eventBatch) to the
	// batchesToTransmit channel. Will mutate eventIdx, eventBatch as a side effect.
	var batch apitype.EngineEventBatch
	transmitBatch := func(ctx context.Context, err chan<- error) {
		if len(batch.Events) == 0 {
			return
		}
		batches <- recordBatchRequest{ctx: ctx, batch: batch, err: err}
		batch = apitype.EngineEventBatch{}
	}

	maxDelayTicker := time.NewTicker(maxTransmissionDelay)

	for {
		select {
		case e, ok := <-events:
			if !ok {
				// Flush any remaining events and close the batch channel. This will cause the workers to drain.
				transmitBatch(context.Background(), nil)
				close(batches)
				return
			}

			batch.Events = append(batch.Events, e.event)
			if len(batch.Events) >= maxEventsToTransmit {
				transmitBatch(e.ctx, e.err)
			} else {
				close(e.err)
			}

		case <-maxDelayTicker.C:
			// If the ticker has fired, send any batched events. This sets an upper bound for
			// the delay between the event being observed and persisted.
			transmitBatch(context.Background(), nil)
		}
	}
}
