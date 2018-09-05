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

package httpstate

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/display"
	"github.com/pulumi/pulumi/pkg/backend/httpstate/client"
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/resource/stack"
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
	done     chan bool
}

func newTokenSource(ctx context.Context, token string, backend *cloudBackend, update client.UpdateIdentifier,
	duration time.Duration) (*tokenSource, error) {

	// Perform an initial lease renewal.
	newToken, err := backend.client.RenewUpdateLease(ctx, update, token, duration)
	if err != nil {
		return nil, err
	}

	requests, done := make(chan tokenRequest), make(chan bool)
	go func() {
		// We will renew the lease after 50% of the duration has elapsed to allow more time for retries.
		ticker := time.NewTicker(duration / 2)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				newToken, err = backend.client.RenewUpdateLease(ctx, update, token, duration)
				if err != nil {
					ticker.Stop()
				} else {
					token = newToken
				}

			case c, ok := <-requests:
				if !ok {
					close(done)
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

	return &tokenSource{requests: requests, done: done}, nil
}

func (ts *tokenSource) Close() {
	close(ts.requests)
	<-ts.done
}

func (ts *tokenSource) GetToken() (string, error) {
	ch := make(chan tokenResponse)
	ts.requests <- ch
	resp := <-ch
	return resp.token, resp.err
}

// cloudUpdate is an implementation of engine.Update backed by remote state and a local program.
type cloudUpdate struct {
	context context.Context
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

func (u *cloudUpdate) Complete(status apitype.UpdateStatus) error {
	defer u.tokenSource.Close()

	token, err := u.tokenSource.GetToken()
	if err != nil {
		return err
	}
	return u.backend.client.CompleteUpdate(u.context, u.update, status, token)
}

func (u *cloudUpdate) recordEvent(
	action apitype.UpdateKind, event engine.Event, seen map[resource.URN]engine.StepEventMetadata,
	opts display.Options) error {

	// If we don't have a token source, we can't perform any mutations.
	if u.tokenSource == nil {
		return nil
	}

	fields := make(map[string]interface{})
	kind := string(apitype.StdoutEvent)
	if event.Type == engine.DiagEvent {
		payload := event.Payload.(engine.DiagEventPayload)
		fields["severity"] = string(payload.Severity)
		if payload.Severity == diag.Error || payload.Severity == diag.Warning {
			kind = string(apitype.StderrEvent)
		}
	}

	// Ensure we render events with raw colorization tags.  Also, render these as 'diff' events so
	// the user has a rich diff-log they can see when the look at their logs in the service.
	opts.Color = colors.Raw
	msg := display.RenderDiffEvent(action, event, seen, opts)
	if msg == "" {
		return nil
	}

	token, err := u.tokenSource.GetToken()
	if err != nil {
		return err
	}

	fields["text"] = msg
	fields["colorize"] = colors.Always
	return u.backend.client.AppendUpdateLogEntry(u.context, u.update, kind, fields, token)
}

func (u *cloudUpdate) RecordAndDisplayEvents(op string, action apitype.UpdateKind,
	events <-chan engine.Event, done chan<- bool, opts display.Options) {

	// Start the local display processor.  Display things however the options have been
	// set to display (i.e. diff vs progress).
	displayEvents := make(chan engine.Event)
	go display.ShowEvents(op, action, displayEvents, done, opts)

	seen := make(map[resource.URN]engine.StepEventMetadata)
	for e := range events {
		// First echo the event to the local display.
		displayEvents <- e

		// Then render and record the event for posterity.
		if err := u.recordEvent(action, e, seen, opts); err != nil {
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

func (b *cloudBackend) newUpdate(ctx context.Context, stackRef backend.StackReference, proj *workspace.Project,
	root string, update client.UpdateIdentifier, token string) (*cloudUpdate, error) {

	// Create a token source for this update if necessary.
	var tokenSource *tokenSource
	if token != "" {
		ts, err := newTokenSource(ctx, token, b, update, 5*time.Minute)
		if err != nil {
			return nil, err
		}
		tokenSource = ts
	}

	// Construct the deployment target.
	target, err := b.getTarget(ctx, stackRef)
	if err != nil {
		return nil, err
	}

	// Construct and return a new update.
	return &cloudUpdate{
		context:     ctx,
		backend:     b,
		update:      update,
		tokenSource: tokenSource,
		root:        root,
		proj:        proj,
		target:      target,
	}, nil
}

func (b *cloudBackend) getSnapshot(ctx context.Context, stackRef backend.StackReference) (*deploy.Snapshot, error) {
	untypedDeployment, err := b.ExportDeployment(ctx, stackRef)
	if err != nil {
		return nil, err
	}

	snapshot, err := stack.DeserializeUntypedDeployment(untypedDeployment)
	if err != nil {
		return nil, err
	}

	return snapshot, nil
}

func (b *cloudBackend) getTarget(ctx context.Context, stackRef backend.StackReference) (*deploy.Target, error) {
	// Pull the local stack info so we can get at its configuration bag.
	stk, err := workspace.DetectProjectStack(stackRef.Name())
	if err != nil {
		return nil, err
	}

	decrypter, err := b.GetStackCrypter(stackRef)
	if err != nil {
		return nil, err
	}
	snapshot, err := b.getSnapshot(ctx, stackRef)
	if err != nil {
		switch err {
		case stack.ErrDeploymentSchemaVersionTooOld:
			return nil, fmt.Errorf("the stack '%s' is too old to be used by this version of the Pulumi CLI",
				stackRef.Name())
		case stack.ErrDeploymentSchemaVersionTooNew:
			return nil, fmt.Errorf("the stack '%s' is newer than what this version of the Pulumi CLI understands. "+
				"Please update your version of the Pulumi CLI", stackRef.Name())
		default:
			return nil, errors.Wrap(err, "could not deserialize deployment")
		}
	}

	return &deploy.Target{
		Name:      stackRef.Name(),
		Config:    stk.Config,
		Decrypter: decrypter,
		Snapshot:  snapshot,
	}, nil
}
