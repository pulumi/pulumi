// Copyright 2025, Pulumi Corporation.
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

package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/pkg/v3/backend/diy/unauthenticatedregistry"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func NewDefaultRegistry(
	ctx context.Context,
	workspace pkgWorkspace.Context,
	project *workspace.Project,
	diag diag.Sink,
	env env.Env,
) registry.Registry {
	return registry.NewOnDemandRegistry(func() (registry.Registry, error) {
		b, err := cmdBackend.NonInteractiveCurrentBackend(
			ctx, workspace, cmdBackend.DefaultLoginManager, project,
		)
		if err == nil && b != nil {
			return b.GetReadOnlyCloudRegistry(), nil
		}
		if b == nil || errors.Is(err, backenderr.ErrLoginRequired) {
			return unauthenticatedregistry.New(diag, env), nil
		}
		return nil, fmt.Errorf("could not get registry backend: %w", err)
	})
}
