// Copyright 2017-2018, Pulumi Corporation.
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

package cloud

import (
	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/operations"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// Stack is a cloud stack.  This simply adds some cloud-specific properties atop the standard backend stack interface.
type Stack interface {
	backend.Stack
	CloudURL() string  // the URL to the cloud containing this stack.
	OrgName() string   // the organization that owns this stack.
	CloudName() string // the PPC in which this stack is running.
	RunLocally() bool  // true if previews/updates/destroys targeting this stack run locally.
}

// cloudStack is a cloud stack descriptor.
type cloudStack struct {
	name      tokens.QName     // the stack's name.
	cloudURL  string           // the URL to the cloud containing this stack.
	orgName   string           // the organization that owns this stack.
	cloudName string           // the PPC in which this stack is running.
	config    config.Map       // the stack's config bag.
	snapshot  *deploy.Snapshot // a snapshot representing the latest deployment state.
	b         *cloudBackend    // a pointer to the backend this stack belongs to.
}

func newStack(apistack apitype.Stack, b *cloudBackend) Stack {
	// Create a fake snapshot out of this stack.
	// TODO[pulumi/pulumi-service#249]: add time, version, etc. info to the manifest.
	stackName := apistack.StackName

	var resources []*resource.State
	for _, res := range apistack.Resources {
		resources = append(resources, resource.NewState(
			res.Type,
			res.URN,
			res.Custom,
			false,
			res.ID,
			resource.NewPropertyMapFromMap(res.Inputs),
			resource.NewPropertyMapFromMap(res.Outputs),
			res.Parent,
			res.Protect,
			// TODO(swgillespie) provide an actual list of dependencies
			[]resource.URN{},
		))
	}
	snapshot := deploy.NewSnapshot(stackName, deploy.Manifest{}, resources)

	// Now assemble all the pieces into a stack structure.
	return &cloudStack{
		name:      stackName,
		cloudURL:  b.CloudURL(),
		orgName:   apistack.OrgName,
		cloudName: apistack.CloudName,
		config:    nil, // TODO[pulumi/pulumi-service#249]: add the config variables.
		snapshot:  snapshot,
		b:         b,
	}
}

// managedCloudName is the name used to refer to the cloud in the Pulumi Service that owns all of an organization's
// managed stacks. All engine operations for a managed stack--previews, updates, destroys, etc.--run locally.
const managedCloudName = "pulumi"

func (s *cloudStack) Name() tokens.QName         { return s.name }
func (s *cloudStack) Config() config.Map         { return s.config }
func (s *cloudStack) Snapshot() *deploy.Snapshot { return s.snapshot }
func (s *cloudStack) Backend() backend.Backend   { return s.b }
func (s *cloudStack) CloudURL() string           { return s.cloudURL }
func (s *cloudStack) OrgName() string            { return s.orgName }
func (s *cloudStack) CloudName() string          { return s.cloudName }
func (s *cloudStack) RunLocally() bool           { return s.cloudName == managedCloudName }

func (s *cloudStack) Remove(force bool) (bool, error) {
	return backend.RemoveStack(s, force)
}

func (s *cloudStack) Preview(proj *workspace.Project, root string,
	debug bool, opts engine.UpdateOptions, displayOpts backend.DisplayOptions) error {
	return backend.PreviewStack(s, proj, root, debug, opts, displayOpts)
}

func (s *cloudStack) Update(proj *workspace.Project, root string,
	debug bool, m backend.UpdateMetadata, opts engine.UpdateOptions, displayOpts backend.DisplayOptions) error {
	return backend.UpdateStack(s, proj, root, debug, m, opts, displayOpts)
}

func (s *cloudStack) Destroy(proj *workspace.Project, root string,
	debug bool, m backend.UpdateMetadata, opts engine.UpdateOptions, displayOpts backend.DisplayOptions) error {
	return backend.DestroyStack(s, proj, root, debug, m, opts, displayOpts)
}

func (s *cloudStack) GetLogs(query operations.LogQuery) ([]operations.LogEntry, error) {
	return backend.GetStackLogs(s, query)
}

func (s *cloudStack) ExportDeployment() (*apitype.UntypedDeployment, error) {
	return backend.ExportStackDeployment(s)
}

func (s *cloudStack) ImportDeployment(deployment *apitype.UntypedDeployment) error {
	return backend.ImportStackDeployment(s, deployment)
}
