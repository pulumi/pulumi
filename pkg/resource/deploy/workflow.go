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

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// workflowType is the type token of the built-in workflow resource.
const workflowType = "pulumi:index:Workflow"

// A WorkflowExecutor runs pulumi:index:Workflow resource operations. Progressing a workflow means
// running nested deployments for its nodes, and the machinery for that (snapshot journaling,
// serialization, callback invocation) lives in pkg/engine, so the executor is injected from there
// via Options.WorkflowExecutor. A nil executor means workflow resources are not supported for the
// current operation.
type WorkflowExecutor interface {
	// Update progresses the workflow: reconcile occupied nodes, admit entries, run the scheduler,
	// then GC removed nodes. olds is nil on create. The returned property map is the workflow's new
	// state; it is valid (and should be persisted) even when an error is returned, in which case
	// the returned status is StatusPartialFailure.
	Update(ctx context.Context, urn resource.URN, olds, news resource.PropertyMap,
	) (resource.PropertyMap, resource.Status, error)

	// Destroy tears down every node's sub-state.
	Destroy(ctx context.Context, urn resource.URN, olds resource.PropertyMap) (resource.Status, error)
}
