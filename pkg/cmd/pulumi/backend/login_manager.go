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

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/diy"
	"github.com/pulumi/pulumi/pkg/v3/backend/diy/postgres"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// LoginManager provides a slim wrapper around functions related to backend logins.
type LoginManager interface {
	// Current returns the currently logged in backend instance for the given url.
	//
	// If the user does not have a logged in backend, then Current will return (nil, nil).
	Current(
		ctx context.Context,
		ws pkgWorkspace.Context,
		sink diag.Sink,
		url string,
		project *workspace.Project,
		setCurrent bool,
	) (backend.Backend, error)

	// Login starts the login process for the given URL. If there is already a logged-in backend, this is returned as-is.
	Login(
		ctx context.Context,
		ws pkgWorkspace.Context,
		sink diag.Sink,
		url string,
		project *workspace.Project,
		setCurrent bool,
		color colors.Colorization,
	) (backend.Backend, error)
}

var DefaultLoginManager LoginManager = &lm{}

type lm struct{}

func (f *lm) Current(
	ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink, url string, project *workspace.Project, setCurrent bool,
) (backend.Backend, error) {
	if diy.IsDIYBackendURL(url) {
		// Handle PostgreSQL backend URLs
		if postgres.IsPostgresBackendURL(url) {
			return postgres.New(ctx, sink, url, project)
		}
		return diy.New(ctx, sink, url, project)
	}

	insecure := pkgWorkspace.GetCloudInsecure(ws, url)
	lm := httpstate.NewLoginManager()
	account, err := lm.Current(ctx, url, insecure, setCurrent)
	if err != nil || account == nil {
		return nil, err
	}
	return httpstate.New(ctx, sink, url, project, insecure)
}

func (f *lm) Login(
	ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink, url string, project *workspace.Project, setCurrent bool,
	color colors.Colorization,
) (backend.Backend, error) {
	if diy.IsDIYBackendURL(url) {
		// Handle PostgreSQL backend URLs
		if postgres.IsPostgresBackendURL(url) {
			if setCurrent {
				return postgres.Login(ctx, sink, url, project)
			}
			return postgres.New(ctx, sink, url, project)
		}

		if setCurrent {
			return diy.Login(ctx, sink, url, project)
		}
		return diy.New(ctx, sink, url, project)
	}

	insecure := pkgWorkspace.GetCloudInsecure(ws, url)
	lm := httpstate.NewLoginManager()
	// Color is the only thing used by lm.Login, so we can just request a colors.Colorization and only fill that part of
	// the display options in. It's hard to change Login itself because it's circularly depended on by esc.
	opts := display.Options{
		Color: color,
	}
	_, err := lm.Login(ctx, url, insecure, "pulumi", "Pulumi stacks", httpstate.WelcomeUser, setCurrent, opts)
	if err != nil {
		return nil, err
	}
	return httpstate.New(ctx, sink, url, project, insecure)
}

type MockLoginManager struct {
	CurrentF func(
		ctx context.Context,
		ws pkgWorkspace.Context,
		sink diag.Sink,
		url string,
		project *workspace.Project,
		setCurrent bool,
	) (backend.Backend, error)

	LoginF func(
		ctx context.Context,
		ws pkgWorkspace.Context,
		sink diag.Sink,
		url string,
		project *workspace.Project,
		setCurrent bool,
		color colors.Colorization,
	) (backend.Backend, error)
}

var _ LoginManager = (*MockLoginManager)(nil)

func (lm *MockLoginManager) Login(
	ctx context.Context,
	ws pkgWorkspace.Context,
	sink diag.Sink,
	url string,
	project *workspace.Project,
	setCurrent bool,
	color colors.Colorization,
) (backend.Backend, error) {
	if lm.LoginF != nil {
		return lm.LoginF(ctx, ws, sink, url, project, setCurrent, color)
	}
	panic("not implemented")
}

func (lm *MockLoginManager) Current(
	ctx context.Context,
	ws pkgWorkspace.Context,
	sink diag.Sink,
	url string,
	project *workspace.Project,
	setCurrent bool,
) (backend.Backend, error) {
	if lm.CurrentF != nil {
		return lm.CurrentF(ctx, ws, sink, url, project, setCurrent)
	}
	panic("not implemented")
}
