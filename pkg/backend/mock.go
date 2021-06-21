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

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/operations"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
)

//
// Mock backend.
//

type MockBackend struct {
	NameF                   func() string
	URLF                    func() string
	GetPolicyPackF          func(ctx context.Context, policyPack string, d diag.Sink) (PolicyPack, error)
	SupportsOrganizationsF  func() bool
	ParseStackReferenceF    func(s string) (StackReference, error)
	ValidateStackNameF      func(s string) error
	DoesProjectExistF       func(context.Context, string) (bool, error)
	GetStackF               func(context.Context, StackReference) (Stack, error)
	CreateStackF            func(context.Context, StackReference, interface{}) (Stack, error)
	RemoveStackF            func(context.Context, Stack, bool) (bool, error)
	ListStacksF             func(context.Context, ListStacksFilter) ([]StackSummary, error)
	RenameStackF            func(context.Context, Stack, tokens.QName) (StackReference, error)
	GetStackCrypterF        func(StackReference) (config.Crypter, error)
	QueryF                  func(context.Context, QueryOperation) result.Result
	GetLatestConfigurationF func(context.Context, Stack) (config.Map, error)
	GetHistoryF             func(context.Context, StackReference, int, int) ([]UpdateInfo, error)
	GetStackTagsF           func(context.Context, Stack) (map[apitype.StackTagName]string, error)
	UpdateStackTagsF        func(context.Context, Stack, map[apitype.StackTagName]string) error
	ExportDeploymentF       func(context.Context, Stack) (*apitype.UntypedDeployment, error)
	ImportDeploymentF       func(context.Context, Stack, *apitype.UntypedDeployment) error
	LogoutF                 func() error
	LogoutAllF              func() error
	CurrentUserF            func() (string, error)
	PreviewF                func(context.Context, Stack,
		UpdateOperation) (engine.ResourceChanges, result.Result)
	UpdateF func(context.Context, Stack,
		UpdateOperation) (engine.ResourceChanges, result.Result)
	ImportF func(context.Context, Stack,
		UpdateOperation, []deploy.Import) (engine.ResourceChanges, result.Result)
	RefreshF func(context.Context, Stack,
		UpdateOperation) (engine.ResourceChanges, result.Result)
	DestroyF func(context.Context, Stack,
		UpdateOperation) (engine.ResourceChanges, result.Result)
	WatchF func(context.Context, Stack,
		UpdateOperation, []string) result.Result
	GetLogsF func(context.Context, Stack, StackConfiguration,
		operations.LogQuery) ([]operations.LogEntry, error)
}

var _ Backend = (*MockBackend)(nil)

func (be *MockBackend) Name() string {
	if be.NameF != nil {
		return be.NameF()
	}
	panic("not implemented")
}

func (be *MockBackend) URL() string {
	if be.URLF != nil {
		return be.URLF()
	}
	panic("not implemented")
}

func (be *MockBackend) ListPolicyGroups(context.Context, string) (apitype.ListPolicyGroupsResponse, error) {
	panic("not implemented")
}

func (be *MockBackend) ListPolicyPacks(context.Context, string) (apitype.ListPolicyPacksResponse, error) {
	panic("not implemented")
}

func (be *MockBackend) GetPolicyPack(
	ctx context.Context, policyPack string, d diag.Sink) (PolicyPack, error) {

	if be.GetPolicyPackF != nil {
		return be.GetPolicyPackF(ctx, policyPack, d)
	}
	panic("not implemented")
}

func (be *MockBackend) SupportsOrganizations() bool {
	if be.SupportsOrganizationsF != nil {
		return be.SupportsOrganizationsF()
	}
	panic("not implemented")
}

func (be *MockBackend) ParseStackReference(s string) (StackReference, error) {
	if be.ParseStackReferenceF != nil {
		return be.ParseStackReferenceF(s)
	}
	panic("not implemented")
}

func (be *MockBackend) ValidateStackName(s string) error {
	if be.ValidateStackNameF != nil {
		return be.ValidateStackNameF(s)
	}
	panic("not implemented")
}

func (be *MockBackend) DoesProjectExist(ctx context.Context, projectName string) (bool, error) {
	if be.DoesProjectExistF != nil {
		return be.DoesProjectExistF(ctx, projectName)
	}
	panic("not implemented")
}

func (be *MockBackend) GetStack(ctx context.Context, stackRef StackReference) (Stack, error) {
	if be.GetStackF != nil {
		return be.GetStackF(ctx, stackRef)
	}
	panic("not implemented")
}

func (be *MockBackend) CreateStack(ctx context.Context, stackRef StackReference, opts interface{}) (Stack, error) {
	if be.CreateStackF != nil {
		return be.CreateStackF(ctx, stackRef, opts)
	}
	panic("not implemented")
}

func (be *MockBackend) RemoveStack(ctx context.Context, stack Stack, force bool) (bool, error) {
	if be.RemoveStackF != nil {
		return be.RemoveStackF(ctx, stack, force)
	}
	panic("not implemented")
}

func (be *MockBackend) ListStacks(ctx context.Context, filter ListStacksFilter) ([]StackSummary, error) {
	if be.ListStacksF != nil {
		return be.ListStacksF(ctx, filter)
	}
	panic("not implemented")
}

func (be *MockBackend) RenameStack(ctx context.Context, stack Stack,
	newName tokens.QName) (StackReference, error) {
	if be.RenameStackF != nil {
		return be.RenameStackF(ctx, stack, newName)
	}
	panic("not implemented")
}

func (be *MockBackend) GetStackCrypter(stackRef StackReference) (config.Crypter, error) {
	if be.GetStackCrypterF != nil {
		return be.GetStackCrypterF(stackRef)
	}
	panic("not implemented")
}

func (be *MockBackend) Preview(ctx context.Context, stack Stack,
	op UpdateOperation) (engine.ResourceChanges, result.Result) {

	if be.PreviewF != nil {
		return be.PreviewF(ctx, stack, op)
	}
	panic("not implemented")
}

func (be *MockBackend) Update(ctx context.Context, stack Stack,
	op UpdateOperation) (engine.ResourceChanges, result.Result) {

	if be.UpdateF != nil {
		return be.UpdateF(ctx, stack, op)
	}
	panic("not implemented")
}

func (be *MockBackend) Import(ctx context.Context, stack Stack,
	op UpdateOperation, imports []deploy.Import) (engine.ResourceChanges, result.Result) {

	if be.ImportF != nil {
		return be.ImportF(ctx, stack, op, imports)
	}
	panic("not implemented")
}

func (be *MockBackend) Refresh(ctx context.Context, stack Stack,
	op UpdateOperation) (engine.ResourceChanges, result.Result) {

	if be.RefreshF != nil {
		return be.RefreshF(ctx, stack, op)
	}
	panic("not implemented")
}

func (be *MockBackend) Destroy(ctx context.Context, stack Stack,
	op UpdateOperation) (engine.ResourceChanges, result.Result) {

	if be.DestroyF != nil {
		return be.DestroyF(ctx, stack, op)
	}
	panic("not implemented")
}

func (be *MockBackend) Watch(ctx context.Context, stack Stack,
	op UpdateOperation, paths []string) result.Result {

	if be.WatchF != nil {
		return be.WatchF(ctx, stack, op, paths)
	}
	panic("not implemented")
}

func (be *MockBackend) Query(ctx context.Context, op QueryOperation) result.Result {

	if be.QueryF != nil {
		return be.QueryF(ctx, op)
	}
	panic("not implemented")
}

func (be *MockBackend) GetHistory(ctx context.Context,
	stackRef StackReference,
	pageSize int,
	page int) ([]UpdateInfo, error) {
	if be.GetHistoryF != nil {
		return be.GetHistoryF(ctx, stackRef, pageSize, page)
	}
	panic("not implemented")
}

func (be *MockBackend) GetLogs(ctx context.Context, stack Stack, cfg StackConfiguration,
	query operations.LogQuery) ([]operations.LogEntry, error) {

	if be.GetLogsF != nil {
		return be.GetLogsF(ctx, stack, cfg, query)
	}
	panic("not implemented")
}

func (be *MockBackend) GetLatestConfiguration(ctx context.Context,
	stack Stack) (config.Map, error) {

	if be.GetLatestConfigurationF != nil {
		return be.GetLatestConfigurationF(ctx, stack)
	}
	panic("not implemented")
}

func (be *MockBackend) GetStackTags(ctx context.Context,
	stack Stack) (map[apitype.StackTagName]string, error) {

	if be.GetStackTagsF != nil {
		return be.GetStackTagsF(ctx, stack)
	}
	panic("not implemented")
}

func (be *MockBackend) UpdateStackTags(ctx context.Context, stack Stack,
	tags map[apitype.StackTagName]string) error {

	if be.UpdateStackTagsF != nil {
		return be.UpdateStackTagsF(ctx, stack, tags)
	}
	panic("not implemented")
}

func (be *MockBackend) ExportDeployment(ctx context.Context,
	stack Stack) (*apitype.UntypedDeployment, error) {

	if be.ExportDeploymentF != nil {
		return be.ExportDeploymentF(ctx, stack)
	}
	panic("not implemented")
}

func (be *MockBackend) ImportDeployment(ctx context.Context, stack Stack,
	deployment *apitype.UntypedDeployment) error {

	if be.ImportDeploymentF != nil {
		return be.ImportDeploymentF(ctx, stack, deployment)
	}
	panic("not implemented")
}

func (be *MockBackend) Logout() error {
	if be.LogoutF != nil {
		return be.LogoutF()
	}
	panic("not implemented")
}

func (be *MockBackend) LogoutAll() error {
	if be.LogoutAllF != nil {
		return be.LogoutAllF()
	}
	panic("not implemented")
}

func (be *MockBackend) CurrentUser() (string, error) {
	if be.CurrentUserF != nil {
		return be.CurrentUserF()
	}
	panic("not implemented")
}

//
// Mock stack.
//

type MockStack struct {
	RefF      func() StackReference
	ConfigF   func() config.Map
	SnapshotF func(ctx context.Context) (*deploy.Snapshot, error)
	BackendF  func() Backend
	PreviewF  func(ctx context.Context, op UpdateOperation) (engine.ResourceChanges, result.Result)
	UpdateF   func(ctx context.Context, op UpdateOperation) (engine.ResourceChanges, result.Result)
	ImportF   func(ctx context.Context, op UpdateOperation,
		imports []deploy.Import) (engine.ResourceChanges, result.Result)
	RefreshF func(ctx context.Context, op UpdateOperation) (engine.ResourceChanges, result.Result)
	DestroyF func(ctx context.Context, op UpdateOperation) (engine.ResourceChanges, result.Result)
	WatchF   func(ctx context.Context, op UpdateOperation, paths []string) result.Result
	QueryF   func(ctx context.Context, op UpdateOperation) result.Result
	RemoveF  func(ctx context.Context, force bool) (bool, error)
	RenameF  func(ctx context.Context, newName tokens.QName) (StackReference, error)
	GetLogsF func(ctx context.Context, cfg StackConfiguration,
		query operations.LogQuery) ([]operations.LogEntry, error)
	ExportDeploymentF func(ctx context.Context) (*apitype.UntypedDeployment, error)
	ImportDeploymentF func(ctx context.Context, deployment *apitype.UntypedDeployment) error
}

var _ Stack = (*MockStack)(nil)

func (ms *MockStack) Ref() StackReference {
	if ms.RefF != nil {
		return ms.RefF()
	}
	panic("not implemented")
}

func (ms *MockStack) Config() config.Map {
	if ms.ConfigF != nil {
		return ms.ConfigF()
	}
	panic("not implemented")
}

func (ms *MockStack) Snapshot(ctx context.Context) (*deploy.Snapshot, error) {
	if ms.SnapshotF != nil {
		return ms.SnapshotF(ctx)
	}
	panic("not implemented")
}

func (ms *MockStack) Backend() Backend {
	if ms.BackendF != nil {
		return ms.BackendF()
	}
	panic("not implemented")
}

func (ms *MockStack) Preview(ctx context.Context, op UpdateOperation) (engine.ResourceChanges, result.Result) {
	if ms.PreviewF != nil {
		return ms.PreviewF(ctx, op)
	}
	panic("not implemented")
}

func (ms *MockStack) Update(ctx context.Context, op UpdateOperation) (engine.ResourceChanges, result.Result) {
	if ms.UpdateF != nil {
		return ms.UpdateF(ctx, op)
	}
	panic("not implemented")
}

func (ms *MockStack) Import(ctx context.Context, op UpdateOperation,
	imports []deploy.Import) (engine.ResourceChanges, result.Result) {
	if ms.ImportF != nil {
		return ms.ImportF(ctx, op, imports)
	}
	panic("not implemented")
}

func (ms *MockStack) Refresh(ctx context.Context, op UpdateOperation) (engine.ResourceChanges, result.Result) {
	if ms.RefreshF != nil {
		return ms.RefreshF(ctx, op)
	}
	panic("not implemented")
}

func (ms *MockStack) Destroy(ctx context.Context, op UpdateOperation) (engine.ResourceChanges, result.Result) {
	if ms.DestroyF != nil {
		return ms.DestroyF(ctx, op)
	}
	panic("not implemented")
}

func (ms *MockStack) Watch(ctx context.Context, op UpdateOperation, paths []string) result.Result {
	if ms.WatchF != nil {
		return ms.WatchF(ctx, op, paths)
	}
	panic("not implemented")
}

func (ms *MockStack) Query(ctx context.Context, op UpdateOperation) result.Result {
	if ms.QueryF != nil {
		return ms.QueryF(ctx, op)
	}
	panic("not implemented")
}

func (ms *MockStack) Remove(ctx context.Context, force bool) (bool, error) {
	if ms.RemoveF != nil {
		return ms.RemoveF(ctx, force)
	}
	panic("not implemented")
}

func (ms *MockStack) Rename(ctx context.Context, newName tokens.QName) (StackReference, error) {
	if ms.RenameF != nil {
		return ms.RenameF(ctx, newName)
	}
	panic("not implemented")
}

func (ms *MockStack) GetLogs(ctx context.Context, cfg StackConfiguration,
	query operations.LogQuery) ([]operations.LogEntry, error) {
	if ms.GetLogsF != nil {
		return ms.GetLogsF(ctx, cfg, query)
	}
	panic("not implemented")
}

func (ms *MockStack) ExportDeployment(ctx context.Context) (*apitype.UntypedDeployment, error) {
	if ms.ExportDeploymentF != nil {
		return ms.ExportDeploymentF(ctx)
	}
	panic("not implemented")
}

func (ms *MockStack) ImportDeployment(ctx context.Context, deployment *apitype.UntypedDeployment) error {
	if ms.ImportDeploymentF != nil {
		return ms.ImportDeploymentF(ctx, deployment)
	}
	panic("not implemented")
}
