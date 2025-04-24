// Copyright 2024, Pulumi Corporation.
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
	"errors"
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/diy"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// BackendInstance is used to inject a backend mock from tests.
var BackendInstance backend.Backend

func IsDIYBackend(ws pkgWorkspace.Context, opts display.Options) (bool, error) {
	if BackendInstance != nil {
		return false, nil
	}

	// Try to read the current project
	project, _, err := ws.ReadProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return false, err
	}

	url, err := pkgWorkspace.GetCurrentCloudURL(ws, env.Global(), project)
	if err != nil {
		return false, fmt.Errorf("could not get cloud url: %w", err)
	}

	return diy.IsDIYBackendURL(url), nil
}

func NonInteractiveCurrentBackend(
	ctx context.Context, ws pkgWorkspace.Context, lm LoginManager, project *workspace.Project,
) (backend.Backend, error) {
	if BackendInstance != nil {
		return BackendInstance, nil
	}

	url, err := pkgWorkspace.GetCurrentCloudURL(ws, env.Global(), project)
	if err != nil {
		return nil, fmt.Errorf("could not get cloud url: %w", err)
	}

	// Only set current if we don't currently have a cloud URL set.
	return lm.Current(ctx, ws, cmdutil.Diag(), url, project, url == "")
}

func CurrentBackend(
	ctx context.Context, ws pkgWorkspace.Context, lm LoginManager, project *workspace.Project,
	opts display.Options,
) (backend.Backend, error) {
	if BackendInstance != nil {
		return BackendInstance, nil
	}

	url, err := pkgWorkspace.GetCurrentCloudURL(ws, env.Global(), project)
	if err != nil {
		return nil, fmt.Errorf("could not get cloud url: %w", err)
	}

	// Only set current if we don't currently have a cloud URL set.
	return lm.Login(ctx, ws, cmdutil.Diag(), url, project, url == "", opts.Color)
}
