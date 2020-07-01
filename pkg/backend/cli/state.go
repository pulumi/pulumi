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
	"time"

	"github.com/pulumi/pulumi/sdk/v2/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/logging"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/v2/backend"
	"github.com/pulumi/pulumi/pkg/v2/backend/display"
	"github.com/pulumi/pulumi/pkg/v2/engine"
	"github.com/pulumi/pulumi/pkg/v2/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v2/resource/stack"
	"github.com/pulumi/pulumi/sdk/v2/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v2/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v2/go/common/workspace"
)

type query struct {
	root string
	proj *workspace.Project
}

func (q *query) GetRoot() string {
	return q.root
}

func (q *query) GetProject() *workspace.Project {
	return q.proj
}

// update is an implementation of engine.Update backed by remote state and a local program.
type update struct {
	context context.Context
	backend *Backend

	update backend.Update

	root   string
	proj   *workspace.Project
	target *deploy.Target
}

func (u *update) GetRoot() string {
	return u.root
}

func (u *update) GetProject() *workspace.Project {
	return u.proj
}

func (u *update) GetTarget() *deploy.Target {
	return u.target
}

func (u *update) Complete(status apitype.UpdateStatus) error {
	return u.update.Complete(u.context, status)
}

// RecordAndDisplayEvents inspects engine events from the given channel, and prints them to the CLI as well as
// posting them to the Pulumi service.
func (u *update) RecordAndDisplayEvents(
	label string, action apitype.UpdateKind, stackID backend.StackIdentifier, op UpdateOperation,
	events <-chan engine.Event, done chan<- bool, opts display.Options, isPreview bool) {
	// We take the channel of engine events and pass them to separate components that will display
	// them to the console or persist them on the Pulumi Service. Both should terminate as soon as
	// they see a CancelEvent, and when finished, close the "done" channel.
	displayEvents := make(chan engine.Event) // Note: unbuffered, but we assume it won't matter in practice.
	displayEventsDone := make(chan bool)

	persistEvents := make(chan engine.Event, 100)
	persistEventsDone := make(chan bool)

	// We close our own done channel when both of the dependent components have finished.
	defer func() {
		<-displayEventsDone
		<-persistEventsDone
		close(done)
	}()

	// Start the Go-routines for displaying and recording events.
	go display.ShowEvents(
		label, action, tokens.QName(stackID.Stack), op.Proj.Name,
		displayEvents, displayEventsDone, opts, isPreview)
	go recordEngineEvents(
		u, opts.Debug, /* persist debug events */
		persistEvents, persistEventsDone)

	for e := range events {
		displayEvents <- e
		persistEvents <- e

		// We stop reading from the event stream as soon as we see the CancelEvent,
		// which will also signal the display/persist components to shutdown too.
		if e.Type == engine.CancelEvent {
			break
		}
	}

	// Note that we don't return immediately, the defer'd function will block until
	// the display and persistence go-routines are finished processing events.
}

func (b *Backend) newQuery(ctx context.Context, op QueryOperation) (engine.QueryInfo, error) {
	return &query{root: op.Root, proj: op.Proj}, nil
}

func (b *Backend) newUpdate(ctx context.Context, stackID backend.StackIdentifier, op UpdateOperation,
	backendUpdate backend.Update) (*update, error) {

	// Construct the deployment target.
	target, err := b.getTarget(ctx, stackID, op.StackConfiguration.Config, op.StackConfiguration.Decrypter)
	if err != nil {
		return nil, err
	}

	// Construct and return a new update.
	return &update{
		context: ctx,
		backend: b,
		update:  backendUpdate,
		root:    op.Root,
		proj:    op.Proj,
		target:  target,
	}, nil
}

func (b *Backend) getSnapshot(ctx context.Context, stackID backend.StackIdentifier) (*deploy.Snapshot, error) {
	untypedDeployment, err := b.exportDeployment(ctx, stackID, nil /* get latest */)
	if err != nil {
		return nil, err
	}

	snapshot, err := stack.DeserializeUntypedDeployment(untypedDeployment, stack.DefaultSecretsProvider)
	if err != nil {
		return nil, err
	}

	return snapshot, nil
}

func (b *Backend) getTarget(ctx context.Context, stackID backend.StackIdentifier,
	cfg config.Map, dec config.Decrypter) (*deploy.Target, error) {

	snapshot, err := b.getSnapshot(ctx, stackID)
	if err != nil {
		switch err {
		case stack.ErrDeploymentSchemaVersionTooOld:
			return nil, fmt.Errorf("the stack '%s' is too old to be used by this version of the Pulumi CLI",
				stackID.Stack)
		case stack.ErrDeploymentSchemaVersionTooNew:
			return nil, fmt.Errorf("the stack '%s' is newer than what this version of the Pulumi CLI understands. "+
				"Please update your version of the Pulumi CLI", stackID.Stack)
		default:
			return nil, errors.Wrap(err, "could not deserialize deployment")
		}
	}

	return &deploy.Target{
		Name:      tokens.QName(stackID.Stack),
		Config:    cfg,
		Decrypter: dec,
		Snapshot:  snapshot,
	}, nil
}

func isDebugDiagEvent(e engine.Event) bool {
	return e.Type == engine.DiagEvent && (e.Payload().(engine.DiagEventPayload)).Severity == diag.Debug
}

// recordEngineEvents reads from a channel of engine events and records them in the client.
func recordEngineEvents(update *update, recordDebugEvents bool, events <-chan engine.Event, done chan<- bool) {
	defer close(done)

	// We maintain a sequence counter for each event to ensure that events can be reconstructured in the same order they
	// were emitted (and not out of order from parallel writes and/or network delays).
	eventIdx := 0

	recordEvent := func(event engine.Event) error {
		apiEvent, err := display.ConvertEngineEvent(event)
		if err != nil {
			return err
		}

		apiEvent.Sequence, eventIdx = eventIdx, eventIdx+1
		apiEvent.Timestamp = int(time.Now().Unix())

		return update.update.RecordEvent(update.context, apiEvent)
	}

	for {
		event, ok := <-events
		if !ok {
			event = engine.NewEvent(engine.CancelEvent, nil)
		}

		// Ignore debug events unless asked to.
		if isDebugDiagEvent(event) && !recordDebugEvents {
			continue
		}

		// Record the event.
		err := recordEvent(event)
		if err != nil {
			logging.V(3).Infof("error recording engine vent: %v", err)
		}

		// Stop processing once we see the CancelEvent.
		if event.Type == engine.CancelEvent {
			return
		}
	}
}
