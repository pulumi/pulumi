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
	"fmt"
	"sync"

	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
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

// A WorkflowProgressor advances pulumi:index:Workflow resources. The deployment executor calls it once
// per update, after the source has completed (so every workflow registration and its step have finished)
// and before deletes are generated. Progressing a workflow means running nested deployments for its
// nodes; that machinery lives in pkg/engine, so the progressor is injected from there via
// Options.WorkflowProgressor. A nil progressor means workflows do not advance.
type WorkflowProgressor interface {
	Progress(ctx context.Context, d *Deployment, host WorkflowHost) error
}

// A WorkflowHost lets a WorkflowProgressor act within the deployment it is progressing.
type WorkflowHost interface {
	// RegisterResource registers a resource as the program would have, runs its step and returns the
	// resulting state.
	RegisterResource(ctx context.Context, goal *pkgresource.Goal) (*pkgresource.State, error)
	// RegisterResourceOutputs records outputs on a registered resource, through the executor's regular
	// path so the change is persisted and displayed like any other outputs registration.
	RegisterResourceOutputs(urn resource.URN, outputs resource.PropertyMap) error
	// Keep marks resources of the previous snapshot as managed by a workflow's nested deployments: the
	// deployment's own delete sweep leaves them alone.
	Keep(urns ...resource.URN)
}

// workflowHost is the executor's WorkflowHost. Its methods may be called concurrently.
type workflowHost struct {
	m  sync.Mutex
	ex *deploymentExecutor
}

func (h *workflowHost) RegisterResource(ctx context.Context, goal *pkgresource.Goal) (*pkgresource.State, error) {
	done := make(chan *RegisterResult, 1)
	h.m.Lock()
	err := h.ex.handleSingleEvent(ctx, &workflowRegisterEvent{goal: goal, done: done})
	h.m.Unlock()
	if err != nil {
		return nil, err
	}
	select {
	case res := <-done:
		if res.Result == ResultStateFailed {
			return nil, fmt.Errorf("registering %v failed", goal.Type)
		}
		return res.State, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (h *workflowHost) RegisterResourceOutputs(urn resource.URN, outputs resource.PropertyMap) error {
	h.m.Lock()
	defer h.m.Unlock()
	return h.ex.stepExec.ExecuteRegisterResourceOutputs(&workflowOutputsEvent{urn: urn, outputs: outputs})
}

func (h *workflowHost) Keep(urns ...resource.URN) {
	h.m.Lock()
	defer h.m.Unlock()
	for _, urn := range urns {
		h.ex.stepGen.kept[urn] = true
	}
}

// workflowRegisterEvent is the synthetic RegisterResourceEvent behind WorkflowHost.RegisterResource.
type workflowRegisterEvent struct {
	goal *pkgresource.Goal
	done chan *RegisterResult
}

func (e *workflowRegisterEvent) event()                             {}
func (e *workflowRegisterEvent) Goal() *pkgresource.Goal            { return e.goal }
func (e *workflowRegisterEvent) Done(result *RegisterResult)        { e.done <- result }
func (e *workflowRegisterEvent) Extension() *apitype.Extension      { return nil }
func (e *workflowRegisterEvent) ExtensionRef() apitype.ExtensionRef { return "" }

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
