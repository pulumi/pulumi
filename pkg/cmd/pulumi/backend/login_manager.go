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
	"fmt"

	pkgauth "github.com/pulumi/pulumi/pkg/v3/auth"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/diy"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// LoginManager provides a slim wrapper around functions related to backend logins.
type LoginManager interface {
	// Session returns the credential helper session this manager prepares and opens backends with.
	Session() *pkgauth.Session

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
		insecure bool,
		color colors.Colorization,
	) (backend.Backend, error)

	LoginFromAuthContext(
		ctx context.Context,
		sink diag.Sink,
		url string,
		project *workspace.Project,
		setCurrent bool,
		insecure bool,
		authContext pkgauth.AuthContext,
	) (backend.Backend, error)
}

// DefaultLoginManager opens backends for the CLI with the process's credential helper session.
var DefaultLoginManager = NewLoginManager(pkgauth.DefaultSession())

// NewLoginManager returns a LoginManager that runs the session's credential helper for the backends
// it opens.
func NewLoginManager(session *pkgauth.Session) LoginManager {
	session.SetBackendValidator(acceptHelperSelection)
	return &lm{session: session}
}

type lm struct {
	session *pkgauth.Session
}

func (f *lm) Session() *pkgauth.Session {
	return f.session
}

func (f *lm) prepare(
	ctx context.Context, ws pkgWorkspace.Context, project *workspace.Project, url string,
) (string, error) {
	if url != "" {
		return f.session.PrepareBackend(ctx, url)
	}
	url, _, err := PrepareCurrentBackend(ctx, f.session, ws, env.Global(), project)
	return url, err
}

func (f *lm) Current(
	ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink, url string, project *workspace.Project, setCurrent bool,
) (backend.Backend, error) {
	url, err := f.prepare(ctx, ws, project, url)
	if err != nil {
		return nil, err
	}
	if diy.IsDIYBackendURL(url) {
		if url == f.session.SelectedBackend() {
			return diy.Login(ctx, sink, url, project)
		}
		return diy.New(ctx, sink, url, project)
	}

	insecure := pkgWorkspace.GetCloudInsecure(ws, url)
	lm := httpstate.NewLoginManagerWithSession(f.session)
	// A backend the helper selected is saved as current, like one the user logged in to.
	credentials, err := lm.Current(ctx, url, insecure, setCurrent || url == f.session.SelectedBackend())
	if err != nil || credentials == nil {
		return nil, err
	}
	return httpstate.NewWithCredentials(ctx, sink, *credentials, project, insecure)
}

func (f *lm) Login(
	ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink, url string, project *workspace.Project, setCurrent bool,
	insecure bool, color colors.Colorization,
) (backend.Backend, error) {
	url, err := f.prepare(ctx, ws, project, url)
	if err != nil {
		return nil, err
	}
	// A backend the helper selected is saved as current, like one the user logged in to.
	setCurrent = setCurrent || url == f.session.SelectedBackend()
	if diy.IsDIYBackendURL(url) {
		if setCurrent {
			return diy.Login(ctx, sink, url, project)
		}
		return diy.New(ctx, sink, url, project)
	}

	lm := httpstate.NewLoginManagerWithSession(f.session)
	// Color is the only display option used by lm.Login.
	opts := display.Options{
		Color: color,
	}
	consoleURL := client.CloudConsoleURL(url)
	welcome := func(opts display.Options) { httpstate.WelcomeUser(opts, consoleURL) }
	credentials, err := lm.Login(ctx, url, insecure, "pulumi", "Pulumi stacks", welcome, setCurrent, opts)
	if err != nil {
		return nil, err
	}
	return httpstate.NewWithCredentials(ctx, sink, *credentials, project, insecure)
}

// LoginFromAuthContext logs in to a backend using the provided authentication context.
// It handles different grant types, such as OIDC token exchange, and returns an error
// for unrecognized grant types.
func (f *lm) LoginFromAuthContext(
	ctx context.Context,
	sink diag.Sink,
	url string,
	project *workspace.Project,
	setCurrent bool,
	insecure bool,
	authContext pkgauth.AuthContext,
) (backend.Backend, error) {
	if authContext.GrantType == pkgauth.AuthContextGrantTypeTokenExchange {
		lm := httpstate.NewLoginManagerWithSession(f.session)
		credentials, err := lm.LoginWithOIDCToken(
			ctx, sink, url, insecure, authContext.Token, authContext.Organization, authContext.Scope,
			authContext.Expiration, setCurrent)
		if err != nil {
			return nil, err
		}
		return httpstate.NewWithCredentials(ctx, sink, *credentials, project, insecure)
	}
	return nil, fmt.Errorf("unknown auth context grant type: %s", authContext.GrantType)
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
		insecure bool,
		color colors.Colorization,
	) (backend.Backend, error)

	LoginFromAuthContextF func(
		ctx context.Context,
		sink diag.Sink,
		url string,
		project *workspace.Project,
		setCurrent bool,
		insecure bool,
		authContext pkgauth.AuthContext,
	) (backend.Backend, error)

	// HelperSession is the session returned by Session. Nil means a session without a helper.
	HelperSession *pkgauth.Session
}

var _ LoginManager = (*MockLoginManager)(nil)

func (lm *MockLoginManager) Session() *pkgauth.Session {
	if lm.HelperSession != nil {
		return lm.HelperSession
	}
	return pkgauth.NewSessionWithHelperFunc(nil)
}

func (lm *MockLoginManager) Login(
	ctx context.Context,
	ws pkgWorkspace.Context,
	sink diag.Sink,
	url string,
	project *workspace.Project,
	setCurrent bool,
	insecure bool,
	color colors.Colorization,
) (backend.Backend, error) {
	if lm.LoginF != nil {
		return lm.LoginF(ctx, ws, sink, url, project, setCurrent, insecure, color)
	}
	panic("not implemented")
}

func (lm *MockLoginManager) LoginFromAuthContext(
	ctx context.Context,
	sink diag.Sink,
	url string,
	project *workspace.Project,
	setCurrent bool,
	insecure bool,
	authContext pkgauth.AuthContext,
) (backend.Backend, error) {
	if lm.LoginFromAuthContextF != nil {
		return lm.LoginFromAuthContextF(ctx, sink, url, project, setCurrent, insecure, authContext)
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
