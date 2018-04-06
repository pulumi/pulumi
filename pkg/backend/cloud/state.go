// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package cloud

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/cloud/client"
	"github.com/pulumi/pulumi/pkg/backend/local"
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/resource/stack"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/workspace"
)

type tokenRequest chan<- tokenResponse

type tokenResponse struct {
	token string
	err   error
}

// tokenSource is a helper type that manages the renewal of the lease token for a managed update.
type tokenSource struct {
	requests chan tokenRequest
}

func newTokenSource(token string, backend *cloudBackend, update client.UpdateIdentifier,
	duration time.Duration) (*tokenSource, error) {

	// Perform an initial lease renewal.
	newToken, err := backend.client.RenewUpdateLease(update, token, duration)
	if err != nil {
		return nil, err
	}

	requests := make(chan tokenRequest)
	go func() {
		// We will renew the lease after 50% of the duration has elapsed to allow more time for retries.
		ticker := time.NewTicker(duration / 2)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				newToken, err = backend.client.RenewUpdateLease(update, token, duration)
				if err != nil {
					ticker.Stop()
				} else {
					token = newToken
				}

			case c, ok := <-requests:
				if !ok {
					return
				}

				resp := tokenResponse{err: err}
				if err == nil {
					resp.token = token
				}
				c <- resp
			}
		}
	}()

	return &tokenSource{requests: requests}, nil
}

func (ts *tokenSource) Close() {
	close(ts.requests)
}

func (ts *tokenSource) GetToken() (string, error) {
	ch := make(chan tokenResponse)
	ts.requests <- ch
	resp := <-ch
	return resp.token, resp.err
}

// cloudUpdate is an implementation of engine.Update backed by remote state and a local program.
type cloudUpdate struct {
	backend *cloudBackend

	update      client.UpdateIdentifier
	tokenSource *tokenSource

	root   string
	proj   *workspace.Project
	target *deploy.Target
}

func (u *cloudUpdate) GetRoot() string {
	return u.root
}

func (u *cloudUpdate) GetProject() *workspace.Project {
	return u.proj
}

func (u *cloudUpdate) GetTarget() *deploy.Target {
	return u.target
}

type cloudStackMutation struct {
	update *cloudUpdate
}

func (u *cloudUpdate) BeginMutation() (engine.SnapshotMutation, error) {
	// invalidate the current checkpoint
	token, err := u.tokenSource.GetToken()
	if err != nil {
		return nil, err
	}
	if err = u.backend.client.InvalidateUpdateCheckpoint(u.update, token); err != nil {
		return nil, err
	}
	return &cloudStackMutation{update: u}, nil
}

func (m *cloudStackMutation) End(snapshot *deploy.Snapshot) error {
	// Upload the new checkpoint.
	token, err := m.update.tokenSource.GetToken()
	if err != nil {
		return err
	}
	deployment := stack.SerializeDeployment(snapshot)
	return m.update.backend.client.PatchUpdateCheckpoint(m.update.update, deployment, token)
}

func (u *cloudUpdate) Complete(status apitype.UpdateStatus) error {
	defer u.tokenSource.Close()

	token, err := u.tokenSource.GetToken()
	if err != nil {
		return err
	}
	return u.backend.client.CompleteUpdate(u.update, status, token)
}

func (u *cloudUpdate) recordEvent(
	event engine.Event, seen map[resource.URN]engine.StepEventMetadata,
	debug bool, opts backend.DisplayOptions, isPreview bool) error {

	// If we don't have a token source, we can't perform any mutations.
	if u.tokenSource == nil {
		return nil
	}

	kind := string(apitype.StdoutEvent)
	if event.Type == engine.DiagEvent {
		payload := event.Payload.(engine.DiagEventPayload)
		if payload.Severity == diag.Error || payload.Severity == diag.Warning {
			kind = string(apitype.StderrEvent)
		}
	}

	// Ensure we render events with raw colorization tags.
	opts.Color = colors.Raw
	msg := local.RenderEvent(event, seen, debug, opts, isPreview)
	if msg == "" {
		return nil
	}

	token, err := u.tokenSource.GetToken()
	if err != nil {
		return err
	}

	fields := map[string]interface{}{"text": msg, "colorize": colors.Always}
	return u.backend.client.AppendUpdateLogEntry(u.update, kind, fields, token)
}

func (u *cloudUpdate) RecordAndDisplayEvents(action string,
	events <-chan engine.Event, done chan<- bool, debug bool, opts backend.DisplayOptions) {

	// Start the local display processor.
	displayEvents := make(chan engine.Event)
	go local.DisplayEvents(action, displayEvents, done, debug, opts)

	seen := make(map[resource.URN]engine.StepEventMetadata)
	isPreview := false
	for e := range events {
		// First echo the event to the local display.
		displayEvents <- e

		if e.Type == engine.PreludeEvent {
			isPreview = e.Payload.(engine.PreludeEventPayload).IsPreview
		}

		// Then render and record the event for posterity.
		if err := u.recordEvent(e, seen, debug, opts, isPreview); err != nil {
			diagEvent := engine.Event{
				Type: engine.DiagEvent,
				Payload: engine.DiagEventPayload{
					Message:  fmt.Sprintf("failed to record event: %v", err),
					Severity: diag.Infoerr,
				},
			}
			displayEvents <- diagEvent
		}

		if e.Type == engine.CancelEvent {
			return
		}
	}
}

func (b *cloudBackend) newUpdate(stackName tokens.QName, proj *workspace.Project, root string,
	update client.UpdateIdentifier, token string) (*cloudUpdate, error) {

	contract.Require(stackName != "", "stackName")

	// Create a token source for this update if necessary.
	var tokenSource *tokenSource
	if token != "" {
		ts, err := newTokenSource(token, b, update, 5*time.Minute)
		if err != nil {
			return nil, err
		}
		tokenSource = ts
	}

	// Construct the deployment target.
	target, err := b.getTarget(stackName)
	if err != nil {
		return nil, err
	}

	// Construct and return a new update.
	return &cloudUpdate{
		backend:     b,
		update:      update,
		tokenSource: tokenSource,
		root:        root,
		proj:        proj,
		target:      target,
	}, nil
}

func (b *cloudBackend) getTarget(stackName tokens.QName) (*deploy.Target, error) {
	// Pull the local stack info so we can get at its configuration bag.
	stk, err := workspace.DetectProjectStack(stackName)
	if err != nil {
		return nil, err
	}

	decrypter, err := b.GetStackCrypter(stackName)
	if err != nil {
		return nil, err
	}

	untypedDeployment, err := b.ExportDeployment(stackName)
	if err != nil {
		return nil, err
	}
	checkpoint := &apitype.CheckpointV1{}
	if err = json.Unmarshal([]byte(untypedDeployment.Deployment), &checkpoint.Latest); err != nil {
		return nil, err
	}
	snapshot, err := stack.DeserializeCheckpoint(checkpoint)
	if err != nil {
		return nil, err
	}

	return &deploy.Target{
		Name:      stackName,
		Config:    stk.Config,
		Decrypter: decrypter,
		Snapshot:  snapshot,
	}, nil
}
