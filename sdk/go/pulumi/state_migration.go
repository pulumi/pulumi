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

// StateMigrationArgs contains the prior state of the resource and its descendants in checkpoint resource format.
//
// This API is experimental and may change.
//
// For the checkpoint resource format, see:
// https://pulumi-developer-docs.readthedocs.io/latest/docs/references/deployment-schema.html#pulumi-resource-state
type StateMigrationArgs struct {
	// URN is the URN of the resource being registered. This may differ from its URN in OldState
	// when the prior resource is matched through an alias.
	URN URN
	// OldState contains the prior state of the resource and its descendants in checkpoint resource format,
	// with the resource itself first. For subsequent callbacks, this includes changes made by earlier callbacks
	// in the chain.
	//
	// For the checkpoint resource format, see:
	// https://pulumi-developer-docs.readthedocs.io/latest/docs/references/deployment-schema.html#pulumi-resource-state
	OldState []map[string]any
}

// StateMigrationResult is returned by a state migration callback when it changes the state.
// Every resource present in the old state must either be returned in NewState under the same URN
// or have an entry in Successors, but not both.
//
// This API is experimental and may change.
type StateMigrationResult struct {
	// NewState contains the complete migrated subtree in checkpoint resource format, including unchanged
	// resources. This replaces [StateMigrationArgs.OldState].
	//
	// For the checkpoint resource format, see:
	// https://pulumi-developer-docs.readthedocs.io/latest/docs/references/deployment-schema.html#pulumi-resource-state
	NewState []map[string]any
	// Successors maps each old URN removed from the state to the URN in NewState that succeeds it.
	// Multiple old URNs may map to the same successor. A resource cannot be removed without a successor.
	// The engine uses these mappings to rewrite resource references.
	Successors map[string]string
}

// StateMigration is the callback signature for the [StateMigrations] resource option.
//
// This API is experimental and may change.
//
// A callback receives the prior state of the resource and its descendants, and may return a replacement subtree
// for the engine to use before diffing those resources. Returning a nil result with no error leaves the callback's
// input state unchanged and allows later callbacks to run. Callbacks must be idempotent.
//
// Migrations run during updates and previews when prior state exists, including state matched through aliases.
// Migrations rewrite state only, they do not create, import, or modify physical resources.
//
// The callback receives plaintext secret values inside their secret envelopes and must not log or otherwise
// expose them. It must not perform Pulumi runtime operations or wait for unresolved Outputs. Every resource omitted
// from the returned state must identify a returned successor. Provider resource states must remain unchanged,
// and custom resources must preserve their physical identity and lifecycle safety flags.
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
