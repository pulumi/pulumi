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

package deploy

import (
	"context"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// workflowType is the type token of the built-in workflow resource.
const workflowType = "pulumi:index:Workflow"

// A WorkflowProgressor advances pulumi:index:Workflow resources. The deployment executor calls it
// once per update, after the source has completed (so every workflow registration and its step have
// finished) and before deletes are generated. Progressing a workflow means running nested, scoped
// deployments for its nodes; that machinery lives in pkg/engine, so the progressor is injected from
// there via Options.WorkflowProgressor. A nil progressor means workflows do not advance.
//
// persistState records a workflow's durable state (cursor positions, entry records) as the
// resource's outputs, through the executor's regular RegisterResourceOutputs path so the change is
// persisted and displayed like any other outputs registration. It may be called at most once per
// registered workflow.
type WorkflowProgressor interface {
	Progress(ctx context.Context, d *Deployment,
		persistState func(urn resource.URN, outputs resource.PropertyMap) error) error
}

// workflowOutputsEvent is the synthetic RegisterResourceOutputsEvent behind persistState.
type workflowOutputsEvent struct {
	urn     resource.URN
	outputs resource.PropertyMap
}

func (e *workflowOutputsEvent) event()                        {}
func (e *workflowOutputsEvent) URN() resource.URN             { return e.urn }
func (e *workflowOutputsEvent) Outputs() resource.PropertyMap { return e.outputs }
func (e *workflowOutputsEvent) Done()                         {}

// MakeWorkflowOwner encodes the owner marking for a resource managed by the given workflow node.
// Node names must not contain '#' (enforced at workflow registration).
func MakeWorkflowOwner(workflow resource.URN, node string) string {
	return string(workflow) + "#" + node
}

// ParseWorkflowOwner decodes an owner marking produced by MakeWorkflowOwner.
func ParseWorkflowOwner(owner string) (workflow resource.URN, node string, ok bool) {
	i := strings.LastIndexByte(owner, '#')
	if i < 0 {
		return "", "", false
	}
	return resource.URN(owner[:i]), owner[i+1:], true
}
