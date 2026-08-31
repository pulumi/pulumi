// Copyright 2026, Pulumi Corporation.
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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/protobuf/proto"
)

// StateMigrationArgs contains the prior checkpoint state passed to a state migration callback.
type StateMigrationArgs struct {
	// URN is the current-program URN of the resource the migration is attached to.
	URN URN
	// OldState contains the prior state of the resource followed by all resources transitively parented to it.
	OldState []map[string]any
}

// StateMigrationResult is returned by a state migration callback when it changes the state.
type StateMigrationResult struct {
	// NewState replaces the complete prior subtree supplied in StateMigrationArgs.OldState.
	NewState []map[string]any
	// Successors maps each omitted prior resource URN to a resource URN present in NewState.
	Successors map[string]string
}

// StateMigration is the callback signature for the StateMigrations resource option.
//
// A migration receives the prior checkpoint state of the resource it is attached to and all resources transitively
// parented to it. It may return a transformed state that the engine splices into its view of prior state before
// diffing. Returning a nil result leaves the state unchanged. Migrations run during normal updates when prior state
// exists and must be idempotent.
//
// Secret values are supplied in plaintext inside their secret envelopes. The callback must only transform the
// supplied state: it must not perform Pulumi runtime operations or wait for unresolved Outputs. Every resource omitted
// from NewState must identify a returned successor, and provider resource states must be returned unchanged.
type StateMigration func(context.Context, *StateMigrationArgs) (*StateMigrationResult, error)

// registerStateMigration starts the callback server if necessary and registers the given migration function.
func (ctx *Context) registerStateMigration(migration StateMigration) (*pulumirpc.Callback, error) {
	if !ctx.state.supportsStateMigrations {
		return nil, errors.New("the Pulumi CLI does not support state migrations. Please update the Pulumi CLI")
	}

	callback := func(innerCtx context.Context, request []byte) (proto.Message, error) {
		var rpcRequest pulumirpc.StateMigrationRequest
		if err := proto.Unmarshal(request, &rpcRequest); err != nil {
			return nil, fmt.Errorf("unmarshaling state migration request: %w", err)
		}

		decoder := json.NewDecoder(bytes.NewReader(rpcRequest.GetOldState()))
		decoder.UseNumber()
		var oldState []map[string]any
		if err := decoder.Decode(&oldState); err != nil {
			return nil, fmt.Errorf("unmarshaling state migration old state: %w", err)
		}

		result, err := migration(innerCtx, &StateMigrationArgs{
			URN:      URN(rpcRequest.GetUrn()),
			OldState: oldState,
		})
		if err != nil {
			return nil, err
		}
		if result == nil {
			return &pulumirpc.StateMigrationResponse{}, nil
		}

		newState, err := json.Marshal(result.NewState)
		if err != nil {
			return nil, fmt.Errorf("marshaling state migration new state: %w", err)
		}
		return &pulumirpc.StateMigrationResponse{
			NewState:   newState,
			Successors: result.Successors,
		}, nil
	}

	err := func() error {
		ctx.state.callbacksLock.Lock()
		defer ctx.state.callbacksLock.Unlock()
		if ctx.state.callbacks == nil {
			callbacks, err := newCallbackServer()
			if err != nil {
				return fmt.Errorf("creating callback server: %w", err)
			}
			ctx.state.callbacks = callbacks
		}
		return nil
	}()
	if err != nil {
		return nil, err
	}

	registered, err := ctx.state.callbacks.RegisterCallback(callback)
	if err != nil {
		return nil, fmt.Errorf("registering callback: %w", err)
	}
	return registered, nil
}
