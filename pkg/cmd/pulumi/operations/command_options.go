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

package operations

import (
	"context"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
)

// CommandOptions lets an embedded CLI supply its workspace and observe stack operations.
type CommandOptions struct {
	Workspace pkgWorkspace.Context
	Run       func(context.Context, backend.Stack, backend.UpdateOperation, chan engine.Event, bool) (
		*deploy.Plan, display.ResourceChanges, error,
	)
}

func commandOptions(opts CommandOptions) CommandOptions {
	if opts.Workspace == nil {
		opts.Workspace = pkgWorkspace.Instance
	}
	if opts.Run == nil {
		opts.Run = RunUpdateOperation
	}
	return opts
}

// RunUpdateOperation runs a preview or an update; it is the default CommandOptions.Run.
func RunUpdateOperation(
	ctx context.Context, s backend.Stack, op backend.UpdateOperation, events chan engine.Event, preview bool,
) (*deploy.Plan, display.ResourceChanges, error) {
	if preview {
		return backend.PreviewStack(ctx, s, op, events)
	}
	changes, err := backend.UpdateStack(ctx, s, op, events)
	return nil, changes, err
}
