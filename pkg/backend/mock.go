// Copyright 2016-2023, Pulumi Corporation.
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
	"archive/tar"
	"bytes"
	"context"
	"iter"
	"slices"
	"strings"
	"time"

	"github.com/blang/semver"
	"github.com/pulumi/esc"
	sdkDisplay "github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/operations"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

//
// Mock backend.
//

type MockBackend struct {
	NameF                  func() string
	URLF                   func() string
	SetCurrentProjectF     func(proj *workspace.Project)
	GetDefaultOrgF         func(ctx context.Context) (string, error)
	GetPolicyPackF         func(ctx context.Context, policyPack string, d diag.Sink) (PolicyPack, error)
	SupportsTagsF          func() bool
	SupportsOrganizationsF func() bool
	SupportsProgressF      func() bool
	ParseStackReferenceF   func(s string) (StackReference, error)
	SupportsDeploymentsF   func() bool
	ValidateStackNameF     func(s string) error
	DoesProjectExistF      func(context.Context, string, string) (bool, error)
	GetStackF              func(context.Context, StackReference) (Stack, error)
	CreateStackF           func(
		context.Context,
		StackReference,
		string,
		*apitype.UntypedDeployment,
		*CreateStackOptions,
	) (Stack, error)
	RemoveStackF func(context.Context, Stack, bool) (bool, error)
	ListStacksF  func(context.Context, ListStacksFilter, ContinuationToken) (
		[]StackSummary, ContinuationToken, error)
	ListStackNamesF func(context.Context, ListStackNamesFilter, ContinuationToken) (
		[]StackReference, ContinuationToken, error)
	RenameStackF                          func(context.Context, Stack, tokens.QName) (StackReference, error)
	GetStackCrypterF                      func(StackReference) (config.Crypter, error)
	GetLatestConfigurationF               func(context.Context, Stack) (config.Map, error)
	GetHistoryF                           func(context.Context, StackReference, int, int) ([]UpdateInfo, error)
	UpdateStackTagsF                      func(context.Context, Stack, map[apitype.StackTagName]string) error
	ExportDeploymentF                     func(context.Context, Stack) (*apitype.UntypedDeployment, error)
	ImportDeploymentF                     func(context.Context, Stack, *apitype.UntypedDeployment) error
	EncryptStackDeploymentSettingsSecretF func(ctx context.Context,
		stack Stack, secret string) (*apitype.SecretValue, error)
	UpdateStackDeploymentSettingsF  func(context.Context, Stack, apitype.DeploymentSettings) error
	DestroyStackDeploymentSettingsF func(ctx context.Context, stack Stack) error
	GetGHAppIntegrationF            func(ctx context.Context, stack Stack) (*apitype.GitHubAppIntegration, error)
	GetStackDeploymentSettingsF     func(context.Context, Stack) (*apitype.DeploymentSettings, error)
	CurrentUserF                    func() (string, []string, *workspace.TokenInformation, error)
	PreviewF                        func(context.Context, Stack,
		UpdateOperation) (*deploy.Plan, sdkDisplay.ResourceChanges, error)
	UpdateF func(context.Context, Stack,
		UpdateOperation) (sdkDisplay.ResourceChanges, error)
	ImportF func(context.Context, Stack,
		UpdateOperation, []deploy.Import) (sdkDisplay.ResourceChanges, error)
	RefreshF func(context.Context, Stack,
		UpdateOperation) (sdkDisplay.ResourceChanges, error)
	DestroyF func(context.Context, Stack,
		UpdateOperation) (sdkDisplay.ResourceChanges, error)
	WatchF func(context.Context, Stack,
		UpdateOperation, []string) error
	GetLogsF func(context.Context, secrets.Provider, Stack, StackConfiguration,
		operations.LogQuery) ([]operations.LogEntry, error)

	CancelCurrentUpdateF func(ctx context.Context, stackRef StackReference) error

	DefaultSecretManagerF func(ps *workspace.ProjectStack) (secrets.Manager, error)

	SupportsTemplatesF        func() bool
	ListTemplatesF            func(_ context.Context, orgName string) (apitype.ListOrgTemplatesResponse, error)
	DownloadTemplateF         func(_ context.Context, orgName, templateSource string) (TarReaderCloser, error)
	GetCloudRegistryF         func() (CloudRegistry, error)
	GetReadOnlyCloudRegistryF func() registry.Registry
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

func (be *MockBackend) SetCurrentProject(project *workspace.Project) {
	if be.SetCurrentProjectF != nil {
		be.SetCurrentProjectF(project)
		return
	}
	panic("not implemented")
}

func (be *MockBackend) ListPolicyGroups(context.Context, string, ContinuationToken) (
	apitype.ListPolicyGroupsResponse, ContinuationToken, error,
) {
	panic("not implemented")
}

func (be *MockBackend) ListPolicyPacks(context.Context, string, ContinuationToken) (
	apitype.ListPolicyPacksResponse, ContinuationToken, error,
) {
	panic("not implemented")
}

func (be *MockBackend) GetPolicyPack(
	ctx context.Context, policyPack string, d diag.Sink,
) (PolicyPack, error) {
	if be.GetPolicyPackF != nil {
		return be.GetPolicyPackF(ctx, policyPack, d)
	}
	panic("not implemented")
}

func (be *MockBackend) SupportsTags() bool {
	if be.SupportsTagsF != nil {
		return be.SupportsTagsF()
	}
	panic("not implemented")
}

func (be *MockBackend) SupportsOrganizations() bool {
	if be.SupportsOrganizationsF != nil {
		return be.SupportsOrganizationsF()
	}
	panic("not implemented")
}

func (be *MockBackend) SupportsProgress() bool {
	if be.SupportsProgressF != nil {
		return be.SupportsProgressF()
	}
	panic("not implemented")
}

func (be *MockBackend) SupportsDeployments() bool {
	if be.SupportsOrganizationsF != nil {
		return be.SupportsDeploymentsF()
	}
	panic("not implemented")
}

func (be *MockBackend) GetDefaultOrg(ctx context.Context) (string, error) {
	if be.GetDefaultOrgF != nil {
		return be.GetDefaultOrgF(ctx)
	}
	return "", nil
}

func (be *MockBackend) ParseStackReference(s string) (StackReference, error) {
	if be.ParseStackReferenceF != nil {
		return be.ParseStackReferenceF(s)
	}

	// default implementation
	split := strings.Split(s, "/")
	var orgName, project, name string
	switch len(split) {
	case 1:
		name = split[0]
	case 2:
		orgName = split[0]
		name = split[1]
	case 3:
		orgName = split[0]
		project = split[1]
		name = split[2]
	}

	parsedName, err := tokens.ParseStackName(name)
	if err != nil {
		return nil, err
	}

	return &MockStackReference{
		StringV:             s,
		NameV:               parsedName,
		ProjectV:            tokens.Name(project),
		OrganizationV:       orgName,
		FullyQualifiedNameV: tokens.QName(s),
	}, nil
}

func (be *MockBackend) ValidateStackName(s string) error {
	if be.ValidateStackNameF != nil {
		return be.ValidateStackNameF(s)
	}
	panic("not implemented")
}

func (be *MockBackend) DoesProjectExist(ctx context.Context, orgName string, projectName string) (bool, error) {
	if be.DoesProjectExistF != nil {
		return be.DoesProjectExistF(ctx, orgName, projectName)
	}
	panic("not implemented")
}

func (be *MockBackend) GetStack(ctx context.Context, stackRef StackReference) (Stack, error) {
	if be.GetStackF != nil {
		return be.GetStackF(ctx, stackRef)
	}
	panic("not implemented")
}

func (be *MockBackend) CreateStack(
	ctx context.Context,
	stackRef StackReference,
	root string,
	initialState *apitype.UntypedDeployment,
	opts *CreateStackOptions,
) (Stack, error) {
	if be.CreateStackF != nil {
		return be.CreateStackF(ctx, stackRef, root, initialState, opts)
	}
	panic("not implemented")
}

func (be *MockBackend) RemoveStack(ctx context.Context, stack Stack, force bool) (bool, error) {
	if be.RemoveStackF != nil {
		return be.RemoveStackF(ctx, stack, force)
	}
	panic("not implemented")
}

func (be *MockBackend) ListStacks(ctx context.Context, filter ListStacksFilter, inContToken ContinuationToken) (
	[]StackSummary, ContinuationToken, error,
) {
	if be.ListStacksF != nil {
		return be.ListStacksF(ctx, filter, inContToken)
	}
	panic("not implemented")
}

func (be *MockBackend) ListStackNames(ctx context.Context, filter ListStackNamesFilter, inContToken ContinuationToken) (
	[]StackReference, ContinuationToken, error,
) {
	if be.ListStackNamesF != nil {
		return be.ListStackNamesF(ctx, filter, inContToken)
	}
	panic("not implemented")
}

func (be *MockBackend) RenameStack(ctx context.Context, stack Stack,
	newName tokens.QName,
) (StackReference, error) {
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
	op UpdateOperation, events chan<- engine.Event,
) (*deploy.Plan, sdkDisplay.ResourceChanges, error) {
	if be.PreviewF != nil {
		return be.PreviewF(ctx, stack, op)
	}
	panic("not implemented")
}

func (be *MockBackend) Update(ctx context.Context, stack Stack,
	op UpdateOperation, events chan<- engine.Event,
) (sdkDisplay.ResourceChanges, error) {
	if be.UpdateF != nil {
		return be.UpdateF(ctx, stack, op)
	}
	panic("not implemented")
}

func (be *MockBackend) Import(ctx context.Context, stack Stack,
	op UpdateOperation, imports []deploy.Import,
) (sdkDisplay.ResourceChanges, error) {
	if be.ImportF != nil {
		return be.ImportF(ctx, stack, op, imports)
	}
	panic("not implemented")
}

func (be *MockBackend) Refresh(ctx context.Context, stack Stack,
	op UpdateOperation,
) (sdkDisplay.ResourceChanges, error) {
	if be.RefreshF != nil {
		return be.RefreshF(ctx, stack, op)
	}
	panic("not implemented")
}

func (be *MockBackend) Destroy(ctx context.Context, stack Stack,
	op UpdateOperation,
) (sdkDisplay.ResourceChanges, error) {
	if be.DestroyF != nil {
		return be.DestroyF(ctx, stack, op)
	}
	panic("not implemented")
}

func (be *MockBackend) Watch(ctx context.Context, stack Stack,
	op UpdateOperation, paths []string,
) error {
	if be.WatchF != nil {
		return be.WatchF(ctx, stack, op, paths)
	}
	panic("not implemented")
}

func (be *MockBackend) GetHistory(ctx context.Context,
	stackRef StackReference,
	pageSize int,
	page int,
) ([]UpdateInfo, error) {
	if be.GetHistoryF != nil {
		return be.GetHistoryF(ctx, stackRef, pageSize, page)
	}
	panic("not implemented")
}

func (be *MockBackend) GetLogs(
	ctx context.Context, secretsProvider secrets.Provider, stack Stack,
	cfg StackConfiguration, query operations.LogQuery,
) ([]operations.LogEntry, error) {
	if be.GetLogsF != nil {
		return be.GetLogsF(ctx, secretsProvider, stack, cfg, query)
	}
	panic("not implemented")
}

func (be *MockBackend) GetLatestConfiguration(ctx context.Context,
	stack Stack,
) (config.Map, error) {
	if be.GetLatestConfigurationF != nil {
		return be.GetLatestConfigurationF(ctx, stack)
	}
	panic("not implemented")
}

func (be *MockBackend) UpdateStackTags(ctx context.Context, stack Stack,
	tags map[apitype.StackTagName]string,
) error {
	if be.UpdateStackTagsF != nil {
		return be.UpdateStackTagsF(ctx, stack, tags)
	}
	panic("not implemented")
}

func (be *MockBackend) ExportDeployment(ctx context.Context,
	stack Stack,
) (*apitype.UntypedDeployment, error) {
	if be.ExportDeploymentF != nil {
		return be.ExportDeploymentF(ctx, stack)
	}
	panic("not implemented")
}

func (be *MockBackend) ImportDeployment(ctx context.Context, stack Stack,
	deployment *apitype.UntypedDeployment,
) error {
	if be.ImportDeploymentF != nil {
		return be.ImportDeploymentF(ctx, stack, deployment)
	}
	panic("not implemented")
}

func (be *MockBackend) CurrentUser() (string, []string, *workspace.TokenInformation, error) {
	if be.CurrentUserF != nil {
		user, org, tokenInfo, err := be.CurrentUserF()
		return user, org, tokenInfo, err
	}
	panic("not implemented")
}

func (be *MockBackend) CancelCurrentUpdate(ctx context.Context, stackRef StackReference) error {
	if be.CancelCurrentUpdateF != nil {
		return be.CancelCurrentUpdateF(ctx, stackRef)
	}
	panic("not implemented")
}

func (be *MockBackend) EncryptStackDeploymentSettingsSecret(
	ctx context.Context, stack Stack, secret string,
) (*apitype.SecretValue, error) {
	if be.EncryptStackDeploymentSettingsSecretF != nil {
		return be.EncryptStackDeploymentSettingsSecretF(ctx, stack, secret)
	}
	panic("not implemented")
}

func (be *MockBackend) UpdateStackDeploymentSettings(ctx context.Context, stack Stack,
	deployment apitype.DeploymentSettings,
) error {
	if be.UpdateStackDeploymentSettingsF != nil {
		return be.UpdateStackDeploymentSettingsF(ctx, stack, deployment)
	}
	panic("not implemented")
}

func (be *MockBackend) GetStackDeploymentSettings(ctx context.Context,
	stack Stack,
) (*apitype.DeploymentSettings, error) {
	if be.GetStackDeploymentSettingsF != nil {
		return be.GetStackDeploymentSettingsF(ctx, stack)
	}
	panic("not implemented")
}

func (be *MockBackend) DestroyStackDeploymentSettings(ctx context.Context, stack Stack) error {
	if be.DestroyStackDeploymentSettingsF != nil {
		return be.DestroyStackDeploymentSettingsF(ctx, stack)
	}
	panic("not implemented")
}

func (be *MockBackend) GetGHAppIntegration(ctx context.Context, stack Stack) (*apitype.GitHubAppIntegration, error) {
	if be.GetGHAppIntegrationF != nil {
		return be.GetGHAppIntegrationF(ctx, stack)
	}
	panic("not implemented")
}

func (be *MockBackend) DefaultSecretManager(ps *workspace.ProjectStack) (secrets.Manager, error) {
	if be.DefaultSecretManagerF != nil {
		return be.DefaultSecretManagerF(ps)
	}
	panic("not implemented")
}

func (be *MockBackend) SupportsTemplates() bool {
	if be.SupportsTemplatesF != nil {
		return be.SupportsTemplatesF()
	}
	panic("not implemented")
}

func (be *MockBackend) ListTemplates(ctx context.Context, orgName string) (apitype.ListOrgTemplatesResponse, error) {
	if be.ListTemplatesF != nil {
		return be.ListTemplatesF(ctx, orgName)
	}
	panic("not implemented")
}

func (be *MockBackend) DownloadTemplate(ctx context.Context, orgName, templateSource string) (TarReaderCloser, error) {
	if be.DownloadTemplateF != nil {
		return be.DownloadTemplateF(ctx, orgName, templateSource)
	}
	panic("not implemented")
}

func (be *MockBackend) GetCloudRegistry() (CloudRegistry, error) {
	if be.GetCloudRegistryF != nil {
		return be.GetCloudRegistryF()
	}
	panic("not implemented")
}

func (be *MockBackend) GetReadOnlyCloudRegistry() registry.Registry {
	if be.GetReadOnlyCloudRegistryF != nil {
		return be.GetReadOnlyCloudRegistryF()
	}
	panic("not implemented")
}

var _ = EnvironmentsBackend((*MockEnvironmentsBackend)(nil))

type MockEnvironmentsBackend struct {
	MockBackend

	CreateEnvironmentF func(
		ctx context.Context,
		org string,
		projectName string,
		envName string,
		yaml []byte,
	) (apitype.EnvironmentDiagnostics, error)

	CheckYAMLEnvironmentF func(
		ctx context.Context,
		org string,
		yaml []byte,
	) (*esc.Environment, apitype.EnvironmentDiagnostics, error)

	OpenYAMLEnvironmentF func(
		ctx context.Context,
		org string,
		yaml []byte,
		duration time.Duration,
	) (*esc.Environment, apitype.EnvironmentDiagnostics, error)
}

func (be *MockEnvironmentsBackend) CreateEnvironment(
	ctx context.Context,
	org string,
	projectName string,
	envName string,
	yaml []byte,
) (apitype.EnvironmentDiagnostics, error) {
	if be.CreateEnvironmentF != nil {
		return be.CreateEnvironmentF(ctx, org, projectName, envName, yaml)
	}
	panic("not implemented")
}

func (be *MockEnvironmentsBackend) CheckYAMLEnvironment(
	ctx context.Context,
	org string,
	yaml []byte,
) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
	if be.CheckYAMLEnvironmentF != nil {
		return be.CheckYAMLEnvironmentF(ctx, org, yaml)
	}
	panic("not implemented")
}

func (be *MockEnvironmentsBackend) OpenYAMLEnvironment(
	ctx context.Context,
	org string,
	yaml []byte,
	duration time.Duration,
) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
	if be.OpenYAMLEnvironmentF != nil {
		return be.OpenYAMLEnvironmentF(ctx, org, yaml, duration)
	}
	panic("not implemented")
}

//
// Mock stack.
//

type MockStack struct {
	RefF                  func() StackReference
	ConfigLocationF       func() StackConfigLocation
	LoadRemoteF           func(ctx context.Context, project *workspace.Project) (*workspace.ProjectStack, error)
	SaveRemoteF           func(ctx context.Context, project *workspace.ProjectStack) error
	OrgNameF              func() string
	ConfigF               func() config.Map
	SnapshotF             func(ctx context.Context, secretsProvider secrets.Provider) (*deploy.Snapshot, error)
	TagsF                 func() map[apitype.StackTagName]string
	BackendF              func() Backend
	DefaultSecretManagerF func(info *workspace.ProjectStack) (secrets.Manager, error)
}

var _ Stack = (*MockStack)(nil)

func (ms *MockStack) Ref() StackReference {
	if ms.RefF != nil {
		return ms.RefF()
	}
	panic("not implemented: MockStack.Ref")
}

func (ms *MockStack) ConfigLocation() StackConfigLocation {
	if ms.ConfigLocationF != nil {
		return ms.ConfigLocationF()
	}
	panic("not implemented: MockStack.HasRemoteConfigF")
}

func (ms *MockStack) LoadRemoteConfig(ctx context.Context, project *workspace.Project,
) (*workspace.ProjectStack, error) {
	if ms.LoadRemoteF != nil {
		return ms.LoadRemoteF(ctx, project)
	}
	panic("not implemented: MockStack.LoadRemote")
}

func (ms *MockStack) SaveRemoteConfig(ctx context.Context, project *workspace.ProjectStack) error {
	if ms.SaveRemoteF != nil {
		return ms.SaveRemoteF(ctx, project)
	}
	panic("not implemented: MockStack.SaveRemote")
}

func (ms *MockStack) OrgName() string {
	if ms.OrgNameF != nil {
		return ms.OrgNameF()
	}
	panic("not implemented: MockStack.OrgName")
}

func (ms *MockStack) Config() config.Map {
	if ms.ConfigF != nil {
		return ms.ConfigF()
	}
	panic("not implemented: MockStack.Config")
}

func (ms *MockStack) Snapshot(ctx context.Context, secretsProvider secrets.Provider) (*deploy.Snapshot, error) {
	if ms.SnapshotF != nil {
		return ms.SnapshotF(ctx, secretsProvider)
	}
	panic("not implemented: MockStack.Snapshot")
}

func (ms *MockStack) Tags() map[apitype.StackTagName]string {
	if ms.TagsF != nil {
		return ms.TagsF()
	}
	panic("not implemented: MockStack.Tags")
}

func (ms *MockStack) Backend() Backend {
	if ms.BackendF != nil {
		return ms.BackendF()
	}
	panic("not implemented: MockStack.Backend")
}

func (ms *MockStack) DefaultSecretManager(info *workspace.ProjectStack) (secrets.Manager, error) {
	if ms.DefaultSecretManagerF != nil {
		return ms.DefaultSecretManagerF(info)
	}
	panic("not implemented: MockStack.DefaultSecretManager")
}

//
// Mock stack reference
//

// MockStackReference is a mock implementation of [StackReference].
// Set the fields on this struct to control the behavior of the mock.
type MockStackReference struct {
	StringV             string
	NameV               tokens.StackName
	ProjectV            tokens.Name
	OrganizationV       string
	FullyQualifiedNameV tokens.QName
}

var _ StackReference = (*MockStackReference)(nil)

func (r *MockStackReference) String() string {
	if r.StringV != "" {
		return r.StringV
	}
	panic("not implemented: MockStackReference.String")
}

func (r *MockStackReference) Name() tokens.StackName {
	if !r.NameV.IsEmpty() {
		return r.NameV
	}
	panic("not implemented: MockStackReference.Name")
}

func (r *MockStackReference) Project() (tokens.Name, bool) {
	if r.ProjectV != "" {
		return r.ProjectV, true
	}
	return "", false
}

func (r *MockStackReference) Organization() (string, bool) {
	if r.OrganizationV != "" {
		return r.OrganizationV, true
	}
	return "", false
}

func (r *MockStackReference) FullyQualifiedName() tokens.QName {
	if r.FullyQualifiedNameV != "" {
		return r.FullyQualifiedNameV
	}
	panic("not implemented: MockStackReference.FullyQualifiedName")
}

type MockPolicyPack struct {
	RefF      func() PolicyPackReference
	BackendF  func() Backend
	PublishF  func(context.Context, PublishOperation) error
	EnableF   func(context.Context, string, PolicyPackOperation) error
	DisableF  func(context.Context, string, PolicyPackOperation) error
	ValidateF func(context.Context, PolicyPackOperation) error
	RemoveF   func(context.Context, PolicyPackOperation) error
}

var _ PolicyPack = (*MockPolicyPack)(nil)

func (mp *MockPolicyPack) Ref() PolicyPackReference {
	if mp.RefF != nil {
		return mp.RefF()
	}
	panic("not implemented: MockPolicyPack.Ref")
}

func (mp *MockPolicyPack) Backend() Backend {
	if mp.BackendF != nil {
		return mp.BackendF()
	}
	panic("not implemented: MockPolicyPack.Backend")
}

func (mp *MockPolicyPack) Publish(ctx context.Context, op PublishOperation) error {
	if mp.PublishF != nil {
		return mp.PublishF(ctx, op)
	}
	panic("not implemented: MockPolicyPack.Publish")
}

func (mp *MockPolicyPack) Enable(ctx context.Context, orgName string, op PolicyPackOperation) error {
	if mp.EnableF != nil {
		return mp.EnableF(ctx, orgName, op)
	}
	panic("not implemented: MockPolicyPack.Enable")
}

func (mp *MockPolicyPack) Disable(ctx context.Context, orgName string, op PolicyPackOperation) error {
	if mp.DisableF != nil {
		return mp.DisableF(ctx, orgName, op)
	}
	panic("not implemented: MockPolicyPack.Disable")
}

func (mp *MockPolicyPack) Validate(ctx context.Context, op PolicyPackOperation) error {
	if mp.ValidateF != nil {
		return mp.ValidateF(ctx, op)
	}
	panic("not implemented: MockPolicyPack.Validate")
}

func (mp *MockPolicyPack) Remove(ctx context.Context, op PolicyPackOperation) error {
	if mp.RemoveF != nil {
		return mp.RemoveF(ctx, op)
	}
	panic("not implemented: MockPolicyPack.Remove")
}

type MockTarReader map[string]MockTarFile

type MockTarFile struct{ Content string }

func (m MockTarReader) Close() error { return nil }

func (m MockTarReader) Tar() *tar.Reader {
	paths := make([]string, 0, len(m))
	for k := range m {
		paths = append(paths, k)
	}
	slices.Sort(paths)

	var b bytes.Buffer
	w := tar.NewWriter(&b)

	for _, p := range paths {
		f := m[p]
		err := w.WriteHeader(&tar.Header{
			Name:     p,
			Size:     int64(len(f.Content)),
			Typeflag: tar.TypeReg,
			Mode:     0o600,
		})
		contract.AssertNoErrorf(err, "impossible")

		_, err = w.Write([]byte(f.Content))
		contract.AssertNoErrorf(err, "impossible")
	}

	contract.AssertNoErrorf(w.Close(), "impossible")
	return tar.NewReader(&b)
}

type MockCloudRegistry struct {
	PublishPackageF  func(context.Context, apitype.PackagePublishOp) error
	PublishTemplateF func(context.Context, apitype.TemplatePublishOp) error
	GetPackageF      func(
		ctx context.Context, source, publisher, name string, version *semver.Version,
	) (apitype.PackageMetadata, error)
	ListPackagesF func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error]
	GetTemplateF  func(
		ctx context.Context, source, publisher, name string, version *semver.Version,
	) (apitype.TemplateMetadata, error)
	ListTemplatesF func(ctx context.Context, name *string) iter.Seq2[apitype.TemplateMetadata, error]
}

var _ CloudRegistry = (*MockCloudRegistry)(nil)

func (mr *MockCloudRegistry) PublishPackage(ctx context.Context, op apitype.PackagePublishOp) error {
	if mr.PublishPackageF != nil {
		return mr.PublishPackageF(ctx, op)
	}
	panic("not implemented: MockCloudRegistry.PublishPackage")
}

func (mr *MockCloudRegistry) GetPackage(
	ctx context.Context, source, publisher, name string, version *semver.Version,
) (apitype.PackageMetadata, error) {
	if mr.GetPackageF != nil {
		return mr.GetPackageF(ctx, source, publisher, name, version)
	}
	panic("not implemented")
}

func (mr *MockCloudRegistry) ListPackages(
	ctx context.Context, name *string,
) iter.Seq2[apitype.PackageMetadata, error] {
	if mr.ListPackagesF != nil {
		return mr.ListPackagesF(ctx, name)
	}
	panic("not implemented")
}

func (mr *MockCloudRegistry) GetTemplate(
	ctx context.Context, source, publisher, name string, version *semver.Version,
) (apitype.TemplateMetadata, error) {
	if mr.GetTemplateF != nil {
		return mr.GetTemplateF(ctx, source, publisher, name, version)
	}
	panic("not implemented")
}

func (mr *MockCloudRegistry) ListTemplates(
	ctx context.Context, name *string,
) iter.Seq2[apitype.TemplateMetadata, error] {
	if mr.ListTemplatesF != nil {
		return mr.ListTemplatesF(ctx, name)
	}
	panic("not implemented")
}

func (mr *MockCloudRegistry) PublishTemplate(ctx context.Context, op apitype.TemplatePublishOp) error {
	if mr.PublishTemplateF != nil {
		return mr.PublishTemplateF(ctx, op)
	}
	panic("not implemented: MockCloudRegistry.PublishTemplate")
}
