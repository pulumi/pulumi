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

package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/pulumi/pulumi/pkg/v2/engine"
	"github.com/pulumi/pulumi/sdk/v2/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v2/go/common/workspace"
)

// ListStacksFilter describes optional filters when listing stacks.
type ListStacksFilter struct {
	Project      *string
	Organization *string
	TagName      *string
	TagValue     *string
}

// StackIdentifier is the set of data needed to identify a Pulumi stack.
type StackIdentifier struct {
	Owner   string
	Project string
	Stack   string
}

// ParseStackIdentifier parses the stack name into a backend.StackIdentifier. Any omitted
// portions will be filled in using the given owner and project.
//
// "alpha"            - will just set the Name, but ignore Owner and Project.
// "alpha/beta"       - will set the Owner and Name, but not Project.
// "alpha/beta/gamma" - will set Owner, Name, and Project.
func ParseStackIdentifier(s, defaultOwner, defaultProject string) (StackIdentifier, error) {
	id := StackIdentifier{
		Owner:   defaultOwner,
		Project: defaultProject,
	}

	split := strings.Split(s, "/")
	switch len(split) {
	case 1:
		id.Stack = split[0]
	case 2:
		id.Owner = split[0]
		id.Stack = split[1]
	case 3:
		id.Owner = split[0]
		id.Project = split[1]
		id.Stack = split[2]
	default:
		return StackIdentifier{}, fmt.Errorf("could not parse stack name '%s'", s)
	}

	return id, nil
}

// ParseStackIdentifierWithClient parses a stack identifier using the client's currently logged-in user and the ambient
// project to fill in default values for Owner and Project when unspecified.
func ParseStackIdentifierWithClient(ctx context.Context, s string, client Client) (StackIdentifier, error) {
	id, err := ParseStackIdentifier(s, "", "")
	if err != nil {
		return StackIdentifier{}, err
	}

	if id.Owner == "" {
		currentUser, err := client.User(ctx)
		if err != nil {
			return StackIdentifier{}, err
		}
		id.Owner = currentUser
	}

	if id.Project == "" {
		currentProject, err := workspace.DetectProject()
		if err != nil {
			return StackIdentifier{}, err
		}
		id.Project = currentProject.Name.String()
	}

	return id, nil
}

func (id StackIdentifier) String() string {
	return fmt.Sprintf("%s/%s/%s", id.Owner, id.Project, id.Stack)
}

// FriendlyName returns the short form of the stack identifier in the context of the given user and project. If the
// project and user both match, they will be elided. If only the project matches, it will be elided. Otherwise, the
// result will be fully-qualified.
func (id StackIdentifier) FriendlyName(currentUser, currentProject string) string {
	// If the project names match or if the stack has no project, we can elide the project name.
	if currentProject == id.Project || id.Project == "" {
		if id.Owner == currentUser || id.Owner == "" {
			return id.Stack // Elide owner too, if it is the current user or if it is the empty string.
		}
		return fmt.Sprintf("%s/%s", id.Owner, id.Stack)
	}

	return fmt.Sprintf("%s/%s/%s", id.Owner, id.Project, id.Stack)
}

// Update tracks an ongoing deployment operation.
type Update interface {
	// ProgressURL returns the URL at which the update's progress can be viewed, if any.
	ProgressURL() string
	// PermalinkURL returns the URL at which the update's logs can be viewed, if any.
	PermalinkURL() string

	// RequiredPolicies returns the policies required by this update.
	RequiredPolicies() []apitype.RequiredPolicy

	// RecordEvent records an engine event associated with the update.
	RecordEvent(ctx context.Context, event apitype.EngineEvent) error
	// PatchCheckpoint submits a new checkpoint object for the update's stack.
	PatchCheckpoint(ctx context.Context, deployment *apitype.DeploymentV3) error
	// Complete marks the update as complete. Calls to RecordEvent and PatchCheckpoint should fail after this method
	// has been called.
	Complete(ctx context.Context, status apitype.UpdateStatus) error
}

// Client implements the low-level operations required by the Pulumi CLI.
type Client interface {
	// Name returns the friendly name for this client.
	Name() string
	// URL returns the URL that this client is connected to.
	URL() string
	// User returns the currently logged-in user.
	User(ctx context.Context) (string, error)
	// DefaultSecretsManager returns the type of secrets manager that is associated with stacks managed by this client,
	// if any.
	DefaultSecretsManager() string

	// DoesProjectExist returns true if the given project exists.
	DoesProjectExist(ctx context.Context, owner, projectName string) (bool, error)
	// StackConsoleURL returns the Pulumi Console URL for the given stack. If the URL is the empty string, callers
	// may assume that the client does not support the Pulumi Console.
	StackConsoleURL(stackID StackIdentifier) (string, error)

	// ListStacks returns a list of stack summaries for all stacks the currently logged-in user is able to view.
	ListStacks(ctx context.Context, filter ListStacksFilter) ([]apitype.StackSummary, error)
	// GetStack returns the metadata for the indicated stack.
	GetStack(ctx context.Context, stackID StackIdentifier) (apitype.Stack, error)
	// CreateStack creates a new stack with the given identifier and tags.
	CreateStack(ctx context.Context, stackID StackIdentifier, tags map[string]string) (apitype.Stack, error)
	// DeleteStack deletes the given stack. If the stack still contains resources, the delete will fail unless the
	// `force` parameter is set to true.
	DeleteStack(ctx context.Context, stackID StackIdentifier, force bool) (bool, error)
	// RenameStack renames the indicated stack to the new identifier.
	RenameStack(ctx context.Context, currentID, newID StackIdentifier) error
	// UpdateStackTags updates the tags for the indicated stack.
	UpdateStackTags(ctx context.Context, stack StackIdentifier, tags map[string]string) error

	// GetStackHistory returns the update information for the given stack.
	GetStackHistory(ctx context.Context, stackID StackIdentifier) ([]apitype.UpdateInfo, error)
	// GetLatestStackConfig returns the latest configuration information for the indicated stack.
	GetLatestStackConfig(ctx context.Context, stackID StackIdentifier) (config.Map, error)
	// ExportStackDeployment exports the statefile for the indicated stack and version. If no version is given, the
	// latest statefile will be returned.
	ExportStackDeployment(ctx context.Context, stackID StackIdentifier, version *int) (apitype.UntypedDeployment, error)
	// ImportStackDeployment imports a statefile into the indicated stack.
	ImportStackDeployment(ctx context.Context, stackID StackIdentifier, deployment *apitype.UntypedDeployment) error

	// StartUpdate starts a new update for the given stack with the indicated metadata.
	StartUpdate(ctx context.Context, kind apitype.UpdateKind, stackID StackIdentifier, proj *workspace.Project,
		cfg config.Map, metadata apitype.UpdateMetadata, opts engine.UpdateOptions, tags map[string]string,
		dryRun bool) (Update, error)
	// CancelCurrentUpdate cancels the currently-running update for a stack, if any.
	CancelCurrentUpdate(ctx context.Context, stackID StackIdentifier) error
}

// PolicyPackIdentifier captures the information necessary to uniquely identify a policy pack.
type PolicyPackIdentifier struct {
	OrgName    string // The name of the organization that administers the policy pack.
	Name       string // The name of the policy pack.
	VersionTag string // The version of the policy pack. Optional.
	ConsoleURL string // The URL of the console for the service that owns this policy pack.
}

func (id PolicyPackIdentifier) String() string {
	return fmt.Sprintf("%s/%s", id.OrgName, id.Name)
}

func (id PolicyPackIdentifier) URL() string {
	return fmt.Sprintf("%s/%s/policypacks/%s", id.ConsoleURL, id.OrgName, id.Name)
}

// ParsePolicyPackIdentifier parses a policy pack identifier in the context of the given user and associates it with
// the given Console URL.
func ParsePolicyPackIdentifier(s, currentUser, consoleURL string) (PolicyPackIdentifier, error) {
	split := strings.Split(s, "/")
	var orgName string
	var policyPackName string

	switch len(split) {
	case 2:
		orgName = split[0]
		policyPackName = split[1]
	default:
		return PolicyPackIdentifier{}, fmt.Errorf("could not parse policy pack name '%s'; must be of the form "+
			"<org-name>/<policy-pack-name>", s)
	}

	if orgName == "" {
		orgName = currentUser
	}

	return PolicyPackIdentifier{
		OrgName:    orgName,
		Name:       policyPackName,
		ConsoleURL: consoleURL,
	}, nil
}

// PolicyClient implements the low-level policy pack operations required by the Pulumi CLI.
type PolicyClient interface {
	// ListPolicyGroups returns all Policy Groups for an organization.
	ListPolicyGroups(ctx context.Context, orgName string) (apitype.ListPolicyGroupsResponse, error)
	// ListPolicyPacks returns all Policy Packs for an organization.
	ListPolicyPacks(ctx context.Context, orgName string) (apitype.ListPolicyPacksResponse, error)

	// GetPolicyPack returns the data for a particular policy pack.
	GetPolicyPack(ctx context.Context, location string) ([]byte, error)
	// GetPolicyPackSchema returns the schema for a particular policy pack.
	GetPolicyPackSchema(ctx context.Context, orgName, policyPackName,
		versionTag string) (*apitype.GetPolicyPackConfigSchemaResponse, error)

	// PublishPolicyPack publishes a policy pack, either creating it anew or adding a new version.
	PublishPolicyPack(ctx context.Context, orgName string, analyzerInfo plugin.AnalyzerInfo,
		dirArchive io.Reader) (string, error)
	// DeletePolicyPack deletes a policy pack, either entirely or only at a specific version.
	DeletePolicyPack(ctx context.Context, orgName, policyPackName, versionTag string) error

	// EnablePolicyPack enables a policy pack for an organization.
	EnablePolicyPack(ctx context.Context, orgName, policyGroup, policyPackName, versionTag string,
		policyPackConfig map[string]*json.RawMessage) error
	// DisablePolicyPack disables a policy pack.
	DisablePolicyPack(ctx context.Context, orgName, policyGroup, policyPackName, versionTag string) error
}
