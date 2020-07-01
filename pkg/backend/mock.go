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
	"encoding/json"
	"io"

	"github.com/pulumi/pulumi/pkg/v2/engine"
	"github.com/pulumi/pulumi/sdk/v2/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v2/go/common/workspace"
)

// Mock client.
type MockClient struct {
	NameF                  func() string
	URLF                   func() string
	UserF                  func(ctx context.Context) (string, error)
	DefaultSecretsManagerF func() string

	DoesProjectExistF func(ctx context.Context, owner, projectName string) (bool, error)
	StackConsoleURLF  func(stackID StackIdentifier) (string, error)

	ListStacksF      func(ctx context.Context, filter ListStacksFilter) ([]apitype.StackSummary, error)
	GetStackF        func(ctx context.Context, stackID StackIdentifier) (apitype.Stack, error)
	CreateStackF     func(ctx context.Context, stackID StackIdentifier, tags map[string]string) (apitype.Stack, error)
	DeleteStackF     func(ctx context.Context, stackID StackIdentifier, force bool) (bool, error)
	RenameStackF     func(ctx context.Context, currentID, newID StackIdentifier) error
	UpdateStackTagsF func(ctx context.Context, stackID StackIdentifier, tags map[string]string) error

	GetStackHistoryF       func(ctx context.Context, stackID StackIdentifier) ([]apitype.UpdateInfo, error)
	GetLatestStackConfigF  func(ctx context.Context, stackID StackIdentifier) (config.Map, error)
	ExportStackDeploymentF func(ctx context.Context, stackID StackIdentifier,
		version *int) (apitype.UntypedDeployment, error)
	ImportStackDeploymentF func(ctx context.Context, stackID StackIdentifier,
		deployment *apitype.UntypedDeployment) error

	StartUpdateF func(ctx context.Context, kind apitype.UpdateKind, stackID StackIdentifier, proj *workspace.Project,
		cfg config.Map, metadata apitype.UpdateMetadata, opts engine.UpdateOptions, tags map[string]string,
		dryRun bool) (Update, error)
	CancelCurrentUpdateF func(ctx context.Context, stackID StackIdentifier) error

	GetPolicyPackF       func(ctx context.Context, location string) ([]byte, error)
	GetPolicyPackSchemaF func(ctx context.Context, orgName, policyPackName,
		versionTag string) (*apitype.GetPolicyPackConfigSchemaResponse, error)

	PublishPolicyPackF func(ctx context.Context, orgName string, analyzerInfo plugin.AnalyzerInfo,
		dirArchive io.Reader) (string, error)
	DeletePolicyPackF func(ctx context.Context, orgName, policyPackName, versionTag string) error

	EnablePolicyPackF func(ctx context.Context, orgName, policyGroup, policyPackName, versionTag string,
		policyPackConfig map[string]*json.RawMessage) error
	DisablePolicyPackF func(ctx context.Context, orgName, policyGroup, policyPackName, versionTag string) error
}

var _ Client = (*MockClient)(nil)

func (c *MockClient) Name() string {
	if c.NameF != nil {
		return c.NameF()
	}
	panic("not implemented")
}

func (c *MockClient) URL() string {
	if c.URLF != nil {
		return c.URLF()
	}
	panic("not implemented")
}

func (c *MockClient) User(ctx context.Context) (string, error) {
	if c.UserF != nil {
		return c.UserF(ctx)
	}
	panic("not implemented")
}

func (c *MockClient) DefaultSecretsManager() string {
	if c.DefaultSecretsManagerF != nil {
		return c.DefaultSecretsManagerF()
	}
	panic("not implemented")
}

func (c *MockClient) DoesProjectExist(ctx context.Context, owner, projectName string) (bool, error) {
	if c.DoesProjectExistF != nil {
		return c.DoesProjectExistF(ctx, owner, projectName)
	}
	panic("not implemented")
}

func (c *MockClient) StackConsoleURL(stackID StackIdentifier) (string, error) {
	if c.StackConsoleURLF != nil {
		return c.StackConsoleURLF(stackID)
	}
	panic("not implemented")
}

func (c *MockClient) ListStacks(ctx context.Context, filter ListStacksFilter) ([]apitype.StackSummary, error) {
	if c.ListStacksF != nil {
		return c.ListStacksF(ctx, filter)
	}
	panic("not implemented")
}

func (c *MockClient) GetStack(ctx context.Context, stackID StackIdentifier) (apitype.Stack, error) {
	if c.GetStackF != nil {
		return c.GetStackF(ctx, stackID)
	}
	panic("not implemented")
}

func (c *MockClient) CreateStack(ctx context.Context, stackID StackIdentifier,
	tags map[string]string) (apitype.Stack, error) {

	if c.CreateStackF != nil {
		return c.CreateStackF(ctx, stackID, tags)
	}
	panic("not implemented")
}

func (c *MockClient) DeleteStack(ctx context.Context, stackID StackIdentifier, force bool) (bool, error) {
	if c.DeleteStackF != nil {
		return c.DeleteStackF(ctx, stackID, force)
	}
	panic("not implemented")
}

func (c *MockClient) RenameStack(ctx context.Context, currentID, newID StackIdentifier) error {
	if c.RenameStackF != nil {
		return c.RenameStackF(ctx, currentID, newID)
	}
	panic("not implemented")
}

func (c *MockClient) UpdateStackTags(ctx context.Context, stackID StackIdentifier, tags map[string]string) error {
	if c.UpdateStackTagsF != nil {
		return c.UpdateStackTagsF(ctx, stackID, tags)
	}
	panic("not implemented")
}

func (c *MockClient) GetStackHistory(ctx context.Context, stackID StackIdentifier) ([]apitype.UpdateInfo, error) {
	if c.GetStackHistoryF != nil {
		return c.GetStackHistoryF(ctx, stackID)
	}
	panic("not implemented")
}

func (c *MockClient) GetLatestStackConfig(ctx context.Context, stackID StackIdentifier) (config.Map, error) {
	if c.GetLatestStackConfigF != nil {
		return c.GetLatestStackConfigF(ctx, stackID)
	}
	panic("not implemented")
}

func (c *MockClient) ExportStackDeployment(ctx context.Context, stackID StackIdentifier,
	version *int) (apitype.UntypedDeployment, error) {

	if c.ExportStackDeploymentF != nil {
		return c.ExportStackDeploymentF(ctx, stackID, version)
	}
	panic("not implemented")
}

func (c *MockClient) ImportStackDeployment(ctx context.Context, stackID StackIdentifier,
	deployment *apitype.UntypedDeployment) error {

	if c.ImportStackDeploymentF != nil {
		return c.ImportStackDeploymentF(ctx, stackID, deployment)
	}
	panic("not implemented")
}

func (c *MockClient) StartUpdate(ctx context.Context, kind apitype.UpdateKind, stackID StackIdentifier,
	proj *workspace.Project, cfg config.Map, metadata apitype.UpdateMetadata, opts engine.UpdateOptions,
	tags map[string]string, dryRun bool) (Update, error) {

	if c.StartUpdateF != nil {
		return c.StartUpdateF(ctx, kind, stackID, proj, cfg, metadata, opts, tags, dryRun)
	}
	panic("not implemented")
}

func (c *MockClient) CancelCurrentUpdate(ctx context.Context, stackID StackIdentifier) error {
	if c.CancelCurrentUpdateF != nil {
		return c.CancelCurrentUpdateF(ctx, stackID)
	}
	panic("not implemented")
}

func (c *MockClient) GetPolicyPack(ctx context.Context, location string) ([]byte, error) {
	if c.GetPolicyPackF != nil {
		return c.GetPolicyPackF(ctx, location)
	}
	panic("not implemented")
}

func (c *MockClient) GetPolicyPackSchema(ctx context.Context, orgName, policyPackName,
	versionTag string) (*apitype.GetPolicyPackConfigSchemaResponse, error) {

	if c.GetPolicyPackSchemaF != nil {
		return c.GetPolicyPackSchemaF(ctx, orgName, policyPackName, versionTag)
	}
	panic("not implemented")
}

func (c *MockClient) PublishPolicyPack(ctx context.Context, orgName string, analyzerInfo plugin.AnalyzerInfo,
	dirArchive io.Reader) (string, error) {

	if c.PublishPolicyPackF != nil {
		return c.PublishPolicyPackF(ctx, orgName, analyzerInfo, dirArchive)
	}
	panic("not implemented")
}

func (c *MockClient) DeletePolicyPack(ctx context.Context, orgName, policyPackName, versionTag string) error {
	if c.DeletePolicyPackF != nil {
		return c.DeletePolicyPackF(ctx, orgName, policyPackName, versionTag)
	}
	panic("not implemented")
}

func (c *MockClient) EnablePolicyPack(ctx context.Context, orgName, policyGroup, policyPackName, versionTag string,
	policyPackConfig map[string]*json.RawMessage) error {

	if c.EnablePolicyPackF != nil {
		return c.EnablePolicyPackF(ctx, orgName, policyGroup, policyPackName, versionTag, policyPackConfig)
	}
	panic("not implemented")
}

func (c *MockClient) DisablePolicyPack(ctx context.Context, orgName, policyGroup, policyPackName,
	versionTag string) error {

	if c.DisablePolicyPackF != nil {
		return c.DisablePolicyPackF(ctx, orgName, policyGroup, policyPackName, versionTag)
	}
	panic("not implemented")
}

// Mock update.
type MockUpdate struct {
	ProgressURLF  func() string
	PermalinkURLF func() string

	RequiredPoliciesF func() []apitype.RequiredPolicy

	RecordEventF     func(ctx context.Context, event apitype.EngineEvent) error
	PatchCheckpointF func(ctx context.Context, deployment *apitype.DeploymentV3) error
	CompleteF        func(ctx context.Context, status apitype.UpdateStatus) error
}

func (u *MockUpdate) ProgressURL() string {
	if u.ProgressURLF != nil {
		return u.ProgressURLF()
	}
	panic("not implemented")
}

func (u *MockUpdate) PermalinkURL() string {
	if u.PermalinkURLF != nil {
		return u.PermalinkURL()
	}
	panic("not implemented")
}

func (u *MockUpdate) RequiredPolicies() []apitype.RequiredPolicy {
	if u.RequiredPoliciesF != nil {
		return u.RequiredPoliciesF()
	}
	panic("not implemented")
}

func (u *MockUpdate) RecordEvent(ctx context.Context, event apitype.EngineEvent) error {
	if u.RecordEventF != nil {
		return u.RecordEventF(ctx, event)
	}
	panic("not implemented")
}

func (u *MockUpdate) PatchCheckpoint(ctx context.Context, deployment *apitype.DeploymentV3) error {
	if u.PatchCheckpointF != nil {
		return u.PatchCheckpointF(ctx, deployment)
	}
	panic("not implemented")
}

func (u *MockUpdate) Complete(ctx context.Context, status apitype.UpdateStatus) error {
	if u.CompleteF != nil {
		return u.CompleteF(ctx, status)
	}
	panic("not implemented")
}
