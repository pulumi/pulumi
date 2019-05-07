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

	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/operations"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/result"
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
	be := &mockBackend{
		ParseStackReferenceF: func(s string) (StackReference, error) {
			return nil, nil
		},
		GetStackF: func(ctx context.Context, stackRef StackReference) (Stack, error) {
			return &mockStack{
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

//
// Mock backend.
//

type mockBackend struct {
	NameF                   func() string
	URLF                    func() string
	ParseStackReferenceF    func(s string) (StackReference, error)
	GetStackF               func(context.Context, StackReference) (Stack, error)
	CreateStackF            func(context.Context, StackReference, interface{}) (Stack, error)
	RemoveStackF            func(context.Context, StackReference, bool) (bool, error)
	ListStacksF             func(context.Context, *tokens.PackageName) ([]StackSummary, error)
	RenameStackF            func(context.Context, StackReference, tokens.QName) error
	GetStackCrypterF        func(StackReference) (config.Crypter, error)
	QueryF                  func(context.Context, StackReference, UpdateOperation) result.Result
	GetLatestConfigurationF func(context.Context, StackReference) (config.Map, error)
	GetHistoryF             func(context.Context, StackReference) ([]UpdateInfo, error)
	GetStackTagsF           func(context.Context, StackReference) (map[apitype.StackTagName]string, error)
	UpdateStackTagsF        func(context.Context, StackReference, map[apitype.StackTagName]string) error
	ExportDeploymentF       func(context.Context, StackReference) (*apitype.UntypedDeployment, error)
	ImportDeploymentF       func(context.Context, StackReference, *apitype.UntypedDeployment) error
	LogoutF                 func() error
	CurrentUserF            func() (string, error)
	PreviewF                func(context.Context, StackReference,
		UpdateOperation) (engine.ResourceChanges, result.Result)
	UpdateF func(context.Context, StackReference,
		UpdateOperation) (engine.ResourceChanges, result.Result)
	RefreshF func(context.Context, StackReference,
		UpdateOperation) (engine.ResourceChanges, result.Result)
	DestroyF func(context.Context, StackReference,
		UpdateOperation) (engine.ResourceChanges, result.Result)
	GetLogsF func(context.Context, StackReference,
		operations.LogQuery) ([]operations.LogEntry, error)
}

var _ Backend = (*mockBackend)(nil)

func (be *mockBackend) Name() string {
	if be.NameF != nil {
		return be.NameF()
	}
	panic("not implemented")
}

func (be *mockBackend) URL() string {
	if be.URLF != nil {
		return be.URLF()
	}
	panic("not implemented")
}

func (be *mockBackend) ParseStackReference(s string) (StackReference, error) {
	if be.ParseStackReferenceF != nil {
		return be.ParseStackReferenceF(s)
	}
	panic("not implemented")
}

func (be *mockBackend) GetStack(ctx context.Context, stackRef StackReference) (Stack, error) {
	if be.GetStackF != nil {
		return be.GetStackF(ctx, stackRef)
	}
	panic("not implemented")
}

func (be *mockBackend) CreateStack(ctx context.Context, stackRef StackReference, opts interface{}) (Stack, error) {
	if be.CreateStackF != nil {
		return be.CreateStackF(ctx, stackRef, opts)
	}
	panic("not implemented")
}

func (be *mockBackend) RemoveStack(ctx context.Context, stackRef StackReference, force bool) (bool, error) {
	if be.RemoveStackF != nil {
		return be.RemoveStackF(ctx, stackRef, force)
	}
	panic("not implemented")
}

func (be *mockBackend) ListStacks(ctx context.Context, projectFilter *tokens.PackageName) ([]StackSummary, error) {
	if be.ListStacksF != nil {
		return be.ListStacksF(ctx, projectFilter)
	}
	panic("not implemented")
}

func (be *mockBackend) RenameStack(ctx context.Context, stackRef StackReference, newName tokens.QName) error {
	if be.RenameStackF != nil {
		return be.RenameStackF(ctx, stackRef, newName)
	}
	panic("not implemented")
}

func (be *mockBackend) GetStackCrypter(stackRef StackReference) (config.Crypter, error) {
	if be.GetStackCrypterF != nil {
		return be.GetStackCrypterF(stackRef)
	}
	panic("not implemented")
}

func (be *mockBackend) Preview(ctx context.Context, stackRef StackReference,
	op UpdateOperation) (engine.ResourceChanges, result.Result) {

	if be.PreviewF != nil {
		return be.PreviewF(ctx, stackRef, op)
	}
	panic("not implemented")
}

func (be *mockBackend) Update(ctx context.Context, stackRef StackReference,
	op UpdateOperation) (engine.ResourceChanges, result.Result) {

	if be.UpdateF != nil {
		return be.UpdateF(ctx, stackRef, op)
	}
	panic("not implemented")
}

func (be *mockBackend) Refresh(ctx context.Context, stackRef StackReference,
	op UpdateOperation) (engine.ResourceChanges, result.Result) {

	if be.RefreshF != nil {
		return be.RefreshF(ctx, stackRef, op)
	}
	panic("not implemented")
}

func (be *mockBackend) Destroy(ctx context.Context, stackRef StackReference,
	op UpdateOperation) (engine.ResourceChanges, result.Result) {

	if be.DestroyF != nil {
		return be.DestroyF(ctx, stackRef, op)
	}
	panic("not implemented")
}

func (be *mockBackend) Query(ctx context.Context, stackRef StackReference,
	op UpdateOperation) result.Result {

	if be.QueryF != nil {
		return be.QueryF(ctx, stackRef, op)
	}
	panic("not implemented")
}

func (be *mockBackend) GetHistory(ctx context.Context, stackRef StackReference) ([]UpdateInfo, error) {
	if be.GetHistoryF != nil {
		return be.GetHistoryF(ctx, stackRef)
	}
	panic("not implemented")
}

func (be *mockBackend) GetLogs(ctx context.Context, stackRef StackReference,
	query operations.LogQuery) ([]operations.LogEntry, error) {

	if be.GetLogsF != nil {
		return be.GetLogsF(ctx, stackRef, query)
	}
	panic("not implemented")
}

func (be *mockBackend) GetLatestConfiguration(ctx context.Context,
	stackRef StackReference) (config.Map, error) {

	if be.GetLatestConfigurationF != nil {
		return be.GetLatestConfigurationF(ctx, stackRef)
	}
	panic("not implemented")
}

func (be *mockBackend) GetStackTags(ctx context.Context,
	stackRef StackReference) (map[apitype.StackTagName]string, error) {

	if be.GetStackTagsF != nil {
		return be.GetStackTagsF(ctx, stackRef)
	}
	panic("not implemented")
}

func (be *mockBackend) UpdateStackTags(ctx context.Context, stackRef StackReference,
	tags map[apitype.StackTagName]string) error {

	if be.UpdateStackTagsF != nil {
		return be.UpdateStackTagsF(ctx, stackRef, tags)
	}
	panic("not implemented")
}

func (be *mockBackend) ExportDeployment(ctx context.Context,
	stackRef StackReference) (*apitype.UntypedDeployment, error) {

	if be.ExportDeploymentF != nil {
		return be.ExportDeploymentF(ctx, stackRef)
	}
	panic("not implemented")
}

func (be *mockBackend) ImportDeployment(ctx context.Context, stackRef StackReference,
	deployment *apitype.UntypedDeployment) error {

	if be.ImportDeploymentF != nil {
		return be.ImportDeploymentF(ctx, stackRef, deployment)
	}
	panic("not implemented")
}

func (be *mockBackend) Logout() error {
	if be.LogoutF != nil {
		return be.LogoutF()
	}
	panic("not implemented")
}

func (be *mockBackend) CurrentUser() (string, error) {
	if be.CurrentUserF != nil {
		return be.CurrentUserF()
	}
	panic("not implemented")
}

//
// Mock stack.
//

type mockStack struct {
	RefF              func() StackReference
	ConfigF           func() config.Map
	SnapshotF         func(ctx context.Context) (*deploy.Snapshot, error)
	BackendF          func() Backend
	PreviewF          func(ctx context.Context, op UpdateOperation) (engine.ResourceChanges, result.Result)
	UpdateF           func(ctx context.Context, op UpdateOperation) (engine.ResourceChanges, result.Result)
	RefreshF          func(ctx context.Context, op UpdateOperation) (engine.ResourceChanges, result.Result)
	DestroyF          func(ctx context.Context, op UpdateOperation) (engine.ResourceChanges, result.Result)
	QueryF            func(ctx context.Context, op UpdateOperation) result.Result
	RemoveF           func(ctx context.Context, force bool) (bool, error)
	RenameF           func(ctx context.Context, newName tokens.QName) error
	GetLogsF          func(ctx context.Context, query operations.LogQuery) ([]operations.LogEntry, error)
	ExportDeploymentF func(ctx context.Context) (*apitype.UntypedDeployment, error)
	ImportDeploymentF func(ctx context.Context, deployment *apitype.UntypedDeployment) error
}

var _ Stack = (*mockStack)(nil)

func (ms *mockStack) Ref() StackReference {
	if ms.RefF != nil {
		return ms.RefF()
	}
	panic("not implemented")
}

func (ms *mockStack) Config() config.Map {
	if ms.ConfigF != nil {
		return ms.ConfigF()
	}
	panic("not implemented")
}

func (ms *mockStack) Snapshot(ctx context.Context) (*deploy.Snapshot, error) {
	if ms.SnapshotF != nil {
		return ms.SnapshotF(ctx)
	}
	panic("not implemented")
}

func (ms *mockStack) Backend() Backend {
	if ms.BackendF != nil {
		return ms.BackendF()
	}
	panic("not implemented")
}

func (ms *mockStack) Preview(ctx context.Context, op UpdateOperation) (engine.ResourceChanges, result.Result) {
	if ms.PreviewF != nil {
		return ms.PreviewF(ctx, op)
	}
	panic("not implemented")
}

func (ms *mockStack) Update(ctx context.Context, op UpdateOperation) (engine.ResourceChanges, result.Result) {
	if ms.UpdateF != nil {
		return ms.UpdateF(ctx, op)
	}
	panic("not implemented")
}

func (ms *mockStack) Refresh(ctx context.Context, op UpdateOperation) (engine.ResourceChanges, result.Result) {
	if ms.RefreshF != nil {
		return ms.RefreshF(ctx, op)
	}
	panic("not implemented")
}

func (ms *mockStack) Destroy(ctx context.Context, op UpdateOperation) (engine.ResourceChanges, result.Result) {
	if ms.DestroyF != nil {
		return ms.DestroyF(ctx, op)
	}
	panic("not implemented")
}

func (ms *mockStack) Query(ctx context.Context, op UpdateOperation) result.Result {
	if ms.QueryF != nil {
		return ms.QueryF(ctx, op)
	}
	panic("not implemented")
}

func (ms *mockStack) Remove(ctx context.Context, force bool) (bool, error) {
	if ms.RemoveF != nil {
		return ms.RemoveF(ctx, force)
	}
	panic("not implemented")
}

func (ms *mockStack) Rename(ctx context.Context, newName tokens.QName) error {
	if ms.RenameF != nil {
		return ms.RenameF(ctx, newName)
	}
	panic("not implemented")
}

func (ms *mockStack) GetLogs(ctx context.Context, query operations.LogQuery) ([]operations.LogEntry, error) {
	if ms.GetLogsF != nil {
		return ms.GetLogsF(ctx, query)
	}
	panic("not implemented")
}

func (ms *mockStack) ExportDeployment(ctx context.Context) (*apitype.UntypedDeployment, error) {
	if ms.ExportDeploymentF != nil {
		return ms.ExportDeploymentF(ctx)
	}
	panic("not implemented")
}

func (ms *mockStack) ImportDeployment(ctx context.Context, deployment *apitype.UntypedDeployment) error {
	if ms.ImportDeploymentF != nil {
		return ms.ImportDeploymentF(ctx, deployment)
	}
	panic("not implemented")
}
