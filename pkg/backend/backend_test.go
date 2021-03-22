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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

func TestGetStackResourceOutputs(t *testing.T) {
	// Create a `backendClient` that consults a (mock) `Backend` to make sure it can get the stack
	// resource outputs correctly.

	typ := "some:invalid:type1"

	resc1 := liveState(typ, "resc1", resource.PropertyMap{
		resource.PropertyKey("prop1"): resource.NewStringProperty("val1")})
	resc2 := liveState(typ, "resc2", resource.PropertyMap{
		resource.PropertyKey("prop2"): resource.NewStringProperty("val2")})

	// `deleted` will be ignored by `GetStackResourceOutputs`.
	deletedName := "resc3"
	deleted := deleteState("deletedType", "resc3", resource.PropertyMap{
		resource.PropertyKey("deleted"): resource.NewStringProperty("deleted")})

	// Mock backend that implements just enough methods to service `GetStackResourceOutputs`.
	// Returns a single stack snapshot.
	be := &MockBackend{
		ParseStackReferenceF: func(s string) (StackReference, error) {
			return nil, nil
		},
		GetStackF: func(ctx context.Context, stackRef StackReference) (Stack, error) {
			return &MockStack{
				SnapshotF: func(ctx context.Context) (*deploy.Snapshot, error) {
					return &deploy.Snapshot{Resources: []*resource.State{
						resc1, resc2, deleted,
					}}, nil
				},
			}, nil
		},
	}

	// Backend client, on which we will call `GetStackResourceOutputs`.
	client := &backendClient{backend: be}

	// Get resource outputs for mock stack.
	outs, err := client.GetStackResourceOutputs(context.Background(), "fakeStack")
	assert.NoError(t, err)

	// Verify resource outputs for resc1.
	resc1Actual, exists := outs[resource.PropertyKey(testURN(typ, "resc1"))]
	assert.True(t, exists)
	assert.True(t, resc1Actual.IsObject())

	resc1Type, exists := resc1Actual.V.(resource.PropertyMap)["type"]
	assert.True(t, exists)
	assert.Equal(t, typ, resc1Type.V)

	resc1Outs, exists := resc1Actual.V.(resource.PropertyMap)["outputs"]
	assert.True(t, exists)
	assert.True(t, resc1Outs.IsObject())

	// Verify resource outputs for resc2.
	resc2Actual, exists := outs[resource.PropertyKey(testURN(typ, "resc2"))]
	assert.True(t, exists)
	assert.True(t, resc2Actual.IsObject())

	resc2Type, exists := resc2Actual.V.(resource.PropertyMap)["type"]
	assert.True(t, exists)
	assert.Equal(t, typ, resc2Type.V) // Same type.

	resc2Outs, exists := resc2Actual.V.(resource.PropertyMap)["outputs"]
	assert.True(t, exists)
	assert.True(t, resc2Outs.IsObject())

	// Verify the deleted resource is not present.
	_, exists = outs[resource.PropertyKey(deletedName)]
	assert.False(t, exists)
}

//
// Helpers.
//

func testURN(typ, name string) resource.URN {
	return resource.NewURN("test", "test", "", tokens.Type(typ), tokens.QName(name))
}

func deleteState(typ, name string, outs resource.PropertyMap) *resource.State {
	return &resource.State{
		Delete: true, Type: tokens.Type(typ), URN: testURN(typ, name), Outputs: outs,
	}
}

func liveState(typ, name string, outs resource.PropertyMap) *resource.State {
	return &resource.State{
		Delete: false, Type: tokens.Type(typ), URN: testURN(typ, name), Outputs: outs,
	}
}
