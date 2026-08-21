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

	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

const (
	// WorkflowType is the type token of the built-in workflow resource.
	WorkflowType tokens.Type = "pulumi:index:Workflow"
	// WorkflowNodeType is the type token of the component resource the engine registers for each node of
	// a workflow. A node's program runs as a nested deployment whose resources are parented under it.
	WorkflowNodeType tokens.Type = "pulumi:index:WorkflowNode"
)

// A WorkflowProgressor advances pulumi:index:Workflow resources. The step executor calls it for a
// workflow's Create or Update step once the step's state is committed and before the program's
// registration completes, so that the workflow's nodes (nested deployments whose resources nest under
// the workflow) are recorded after it and the outputs the program receives are those of this run. That
// machinery lives in pkg/engine, so the progressor is injected from there via Options.WorkflowProgressor.
// A nil progressor means workflows do not advance.
type WorkflowProgressor interface {
	Progress(ctx context.Context, d *Deployment, wf *pkgresource.State, host WorkflowHost) error
}

// A WorkflowHost lets a WorkflowProgressor act within the deployment it is progressing.
type WorkflowHost interface {
	// RegisterResourceOutputs records the workflow's outputs through the executor's regular path, so the
	// change is persisted and displayed like any other outputs registration and the program sees it.
	RegisterResourceOutputs(urn resource.URN, outputs resource.PropertyMap) error
	// Keep marks resources of the previous snapshot as managed by a workflow's nested deployments: the
	// deployment's own delete sweep leaves them alone.
	Keep(urns ...resource.URN)
}

// workflowHost is the step executor's WorkflowHost.
type workflowHost struct {
	se *stepExecutor
}

func (h *workflowHost) RegisterResourceOutputs(urn resource.URN, outputs resource.PropertyMap) error {
	return h.se.ExecuteRegisterResourceOutputs(&workflowOutputsEvent{urn: urn, outputs: outputs})
}

func (h *workflowHost) Keep(urns ...resource.URN) {
	h.se.deployment.Keep(urns...)
}

// workflowOutputsEvent is the synthetic RegisterResourceOutputsEvent behind
// WorkflowHost.RegisterResourceOutputs.
type workflowOutputsEvent struct {
	urn     resource.URN
	outputs resource.PropertyMap
}

func (e *workflowOutputsEvent) event()                        {}
func (e *workflowOutputsEvent) URN() resource.URN             { return e.urn }
func (e *workflowOutputsEvent) Outputs() resource.PropertyMap { return e.outputs }
func (e *workflowOutputsEvent) Done()                         {}
