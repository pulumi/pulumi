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

package newcmd

import (
	"context"
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// appendTemplateEnvironments mutates ps to import the given ESC environments,
// skipping any that are already imported. Returns the list of envs that were
// actually added (in input order), suitable for use in a status message.
func appendTemplateEnvironments(ps *workspace.ProjectStack, envs []string) []string {
	if len(envs) == 0 {
		return nil
	}

	existing := map[string]bool{}
	if ps.Environment != nil {
		for _, e := range ps.Environment.Imports() {
			existing[e] = true
		}
	}

	var toAdd []string
	for _, e := range envs {
		if existing[e] {
			continue
		}
		existing[e] = true
		toAdd = append(toAdd, e)
	}
	if len(toAdd) == 0 {
		return nil
	}

	if ps.Environment == nil {
		ps.Environment = workspace.NewEnvironment(toAdd)
	} else {
		ps.Environment = ps.Environment.Append(toAdd...)
	}
	return toAdd
}

// applyTemplateEnvironments loads the stack's project stack file, appends the
// given ESC environments as imports, and saves it. It is a no-op when envs is
// empty or every entry is already present.
func applyTemplateEnvironments(
	ctx context.Context,
	sink diag.Sink,
	project *workspace.Project,
	stack backend.Stack,
	envs []string,
) ([]string, error) {
	if len(envs) == 0 {
		return nil, nil
	}

	ps, err := cmdStack.LoadProjectStack(ctx, sink, project, stack)
	if err != nil {
		return nil, fmt.Errorf("loading stack config: %w", err)
	}

	added := appendTemplateEnvironments(ps, envs)
	if len(added) == 0 {
		return nil, nil
	}

	if err := cmdStack.SaveProjectStack(ctx, stack, ps); err != nil {
		return nil, fmt.Errorf("saving stack config: %w", err)
	}
	return added, nil
}
