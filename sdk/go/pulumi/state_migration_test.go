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
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

func TestStateMigrationCallback(t *testing.T) {
	t.Parallel()

	const (
		componentURN = "urn:pulumi:stack::project::example:index:Component::component"
		oldChildURN  = componentURN + "$example:index:Old::child"
		newChildURN  = componentURN + "$example:index:New::child"
		oldStateJSON = `[
			{"urn":"` + componentURN + `","type":"example:index:Component"},
			{"urn":"` + oldChildURN + `","type":"example:index:Old","outputs":{"large":9007199254740993}}
		]`
	)

	callbacks := &callbackServer{functions: map[string]callbackFunction{}}
	ctx := &Context{state: &contextState{
		supportsStateMigrations: true,
		callbacks:               callbacks,
	}}

	migration, err := ctx.registerStateMigration(func(
		_ context.Context, args *StateMigrationArgs,
	) (*StateMigrationResult, error) {
		assert.Equal(t, URN(componentURN), args.URN)
		assert.Equal(t, json.Number("9007199254740993"), args.OldState[1]["outputs"].(map[string]any)["large"])

		args.OldState[1]["urn"] = newChildURN
		args.OldState[1]["type"] = "example:index:New"
		return &StateMigrationResult{
			NewState: args.OldState,
			Successors: map[string]string{
				oldChildURN: newChildURN,
			},
		}, nil
	})
	require.NoError(t, err)

	request, err := proto.Marshal(&pulumirpc.StateMigrationRequest{
		Urn:      componentURN,
		OldState: []byte(oldStateJSON),
	})
	require.NoError(t, err)

	response, err := callbacks.Invoke(t.Context(), &pulumirpc.CallbackInvokeRequest{
		Token:   migration.Token,
		Request: request,
	})
	require.NoError(t, err)

	var migrationResponse pulumirpc.StateMigrationResponse
	require.NoError(t, proto.Unmarshal(response.Response, &migrationResponse))
	assert.Contains(t, string(migrationResponse.NewState), "9007199254740993")
	assert.Equal(t, map[string]string{oldChildURN: newChildURN}, migrationResponse.Successors)

	var newState []map[string]any
	require.NoError(t, json.Unmarshal(migrationResponse.NewState, &newState))
	assert.Equal(t, newChildURN, newState[1]["urn"])
}

func TestStateMigrationCallbackNoop(t *testing.T) {
	t.Parallel()

	callbacks := &callbackServer{functions: map[string]callbackFunction{}}
	ctx := &Context{state: &contextState{
		supportsStateMigrations: true,
		callbacks:               callbacks,
	}}
	migration, err := ctx.registerStateMigration(func(
		context.Context, *StateMigrationArgs,
	) (*StateMigrationResult, error) {
		return nil, nil
	})
	require.NoError(t, err)

	request, err := proto.Marshal(&pulumirpc.StateMigrationRequest{OldState: []byte("[]")})
	require.NoError(t, err)
	response, err := callbacks.Invoke(t.Context(), &pulumirpc.CallbackInvokeRequest{
		Token:   migration.Token,
		Request: request,
	})
	require.NoError(t, err)

	var migrationResponse pulumirpc.StateMigrationResponse
	require.NoError(t, proto.Unmarshal(response.Response, &migrationResponse))
	assert.Empty(t, migrationResponse.NewState)
	assert.Empty(t, migrationResponse.Successors)
}

func TestStateMigrationRequiresFeatureSupport(t *testing.T) {
	t.Parallel()

	ctx := &Context{state: &contextState{}}
	_, err := ctx.registerStateMigration(func(
		context.Context, *StateMigrationArgs,
	) (*StateMigrationResult, error) {
		return nil, nil
	})
	require.ErrorContains(t, err, "does not support state migrations")
}

func TestStateMigrationsResourceOption(t *testing.T) {
	t.Parallel()

	var calls []string
	first := func(context.Context, *StateMigrationArgs) (*StateMigrationResult, error) {
		calls = append(calls, "first")
		return nil, nil
	}
	second := func(context.Context, *StateMigrationArgs) (*StateMigrationResult, error) {
		calls = append(calls, "second")
		return nil, nil
	}

	opts, err := NewResourceOptions(
		StateMigrations([]StateMigration{first}),
		StateMigrations([]StateMigration{second}),
	)
	require.NoError(t, err)
	require.Len(t, opts.StateMigrations, 2)
	for _, migration := range opts.StateMigrations {
		_, err := migration(t.Context(), &StateMigrationArgs{})
		require.NoError(t, err)
	}
	assert.Equal(t, []string{"first", "second"}, calls)
}

func TestStateMigrationsSentWithRegistration(t *testing.T) {
	t.Parallel()

	var registered []*pulumirpc.Callback
	mocks := &testMonitor{
		NewResourceF: func(args MockResourceArgs) (string, resource.PropertyMap, error) {
			if args.Name == "component" {
				registered = args.RegisterRPC.GetStateMigrations()
			}
			return args.Name, resource.PropertyMap{}, nil
		},
	}

	err := RunErr(func(ctx *Context) error {
		ctx.state.callbacks = &callbackServer{functions: map[string]callbackFunction{}}
		var component testComp
		return ctx.RegisterComponentResource(
			"example:index:Component",
			"component",
			&component,
			StateMigrations([]StateMigration{
				func(context.Context, *StateMigrationArgs) (*StateMigrationResult, error) {
					return nil, nil
				},
			}),
		)
	}, WithMocks("project", "stack", mocks))
	require.NoError(t, err)
	require.Len(t, registered, 1)
	assert.NotEmpty(t, registered[0].GetToken())
}
