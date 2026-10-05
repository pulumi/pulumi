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
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/httputil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"

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
	// ResolveCredentials prepares available credentials without validating an account or logging in.
	// An empty URL resolves the current backend; a nonempty URL cannot be changed by the helper.
	ResolveCredentials(context.Context, pkgWorkspace.Context, string, *workspace.Project) (*pkgauth.Credentials, error)

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
		options LoginOptions,
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

// LoginOptions controls connection settings and optional helper configuration for a login.
type LoginOptions struct {
	SetCurrent bool
	Insecure   bool
	Color      colors.Colorization
	Helper     *workspace.CredentialHelper
}

// DefaultLoginManager opens backends for the CLI with the process's credential helper session.
var DefaultLoginManager = NewLoginManager(pkgauth.DefaultSession())

// NewLoginManager returns a LoginManager that prepares backends with the session's optional credential helper.
func NewLoginManager(session *pkgauth.Session) LoginManager {
	return &lm{session: session, credentials: map[string]*pkgauth.Credentials{}}
}

type lm struct {
	session     *pkgauth.Session
	mu          sync.Mutex
	credentials map[string]*pkgauth.Credentials
}

func (f *lm) ResolveCredentials(ctx context.Context, ws pkgWorkspace.Context, url string,
	project *workspace.Project,
) (*pkgauth.Credentials, error) {
	url, _, err := f.prepare(ctx, ws, project, url)
	if err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if credentials := f.credentials[url]; credentials != nil {
		return credentials, nil
	}
	credentials := &pkgauth.Credentials{BackendURL: url}
	if helperAuth := f.session.HelperHTTPAuth(url); helperAuth != nil {
		credentials.HTTPAuth = helperAuth
		credentials.Insecure = pkgWorkspace.GetCloudInsecure(ws, url)
	} else if pkgauth.IsHTTPBackend(url) {
		credentials, err = httpstate.LoadCredentials(url)
		if err != nil {
			return nil, err
		}
	}
	f.describeCredentials(credentials)
	f.credentials[url] = credentials
	return credentials, nil
}

func (f *lm) describeCredentials(credentials *pkgauth.Credentials) {
	info := f.session.CredentialInfo(credentials.BackendURL)
	if info.AccessTokenSource == "" {
		info.AccessTokenSource = "none"
		if credentials.HTTPAuth.AccessToken() != "" {
			info.AccessTokenSource = "account credentials"
			if env.AccessToken.Value() != "" {
				info.AccessTokenSource = "PULUMI_ACCESS_TOKEN"
			}
		}
	}
	credentials.Info = info
}

func (f *lm) rememberCredentials(credentials *pkgauth.Credentials) {
	f.describeCredentials(credentials)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.credentials[credentials.BackendURL] = credentials
}

func (f *lm) prepare(
	ctx context.Context, ws pkgWorkspace.Context, project *workspace.Project, url string,
) (string, bool, error) {
	if url != "" {
		logging.AddGlobalSecretFilter(httputil.URLSecrets(url), "[credential]")
		url, err := f.session.PrepareBackend(ctx, url)
		return url, false, err
	}
	return prepareCurrentBackend(ctx, f.session, ws, env.Global(), project)
}

func (f *lm) Current(
	ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink, url string, project *workspace.Project, setCurrent bool,
) (backend.Backend, error) {
	url, preparedCurrent, err := f.prepare(ctx, ws, project, url)
	if err != nil {
		return nil, err
	}
	setCurrent = setCurrent || preparedCurrent
	if diy.IsDIYBackendURL(url) {
		if setCurrent {
			return diy.Login(ctx, sink, url, project)
		}
		return diy.New(ctx, sink, url, project)
	}

	insecure := pkgWorkspace.GetCloudInsecure(ws, url)
	credentials, err := httpstate.NewLoginManager(f.session).Current(ctx, url, insecure, setCurrent)
	if err != nil || credentials == nil {
		return nil, err
	}
	f.rememberCredentials(credentials)
	return httpstate.NewWithCredentials(ctx, sink, *credentials, project)
}

func (f *lm) Login(
	ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink, url string, project *workspace.Project,
	options LoginOptions,
) (backend.Backend, error) {
	url, preparedCurrent, err := prepareLogin(ctx, f.session, ws, url, project, options)
	options.SetCurrent = options.SetCurrent || preparedCurrent
	if err != nil {
		return nil, err
	}
	var be backend.Backend
	if diy.IsDIYBackendURL(url) {
		if options.SetCurrent {
			be, err = diy.Login(ctx, sink, url, project)
		} else {
			be, err = diy.New(ctx, sink, url, project)
		}
	} else {
		opts := display.Options{Color: options.Color}
		consoleURL := client.CloudConsoleURL(url)
		welcome := func(opts display.Options) { httpstate.WelcomeUser(opts, consoleURL) }
		var credentials *pkgauth.Credentials
		credentials, err = httpstate.NewLoginManager(f.session).Login(
			ctx, url, options.Insecure, "pulumi", "Pulumi stacks", welcome, options.SetCurrent, opts)
		if err == nil {
			f.rememberCredentials(credentials)
			be, err = httpstate.NewWithCredentials(ctx, sink, *credentials, project)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("logging in to %s: %w", url, err)
	}
	if options.Helper != nil {
		helper, err := f.session.ResolveHelper()
		if err != nil {
			return nil, err
		}
		if err := pkgauth.SaveHelper(&helper.CredentialHelper); err != nil {
			return nil, err
		}
	}
	return be, nil
}

func prepareLogin(ctx context.Context, session *pkgauth.Session, ws pkgWorkspace.Context, url string,
	project *workspace.Project, options LoginOptions,
) (string, bool, error) {
	if options.Helper != nil {
		if err := session.UseHelper(*options.Helper); err != nil {
			return "", false, err
		}
	}
	if url != "" {
		url, err := session.PrepareBackend(ctx, url)
		return url, false, err
	}
	fallback, err := pkgWorkspace.GetCurrentCloudURLWithAgentFallback(ws, env.Global(), project)
	if err != nil {
		return "", false, err
	}
	setCurrent := fallback == ""
	if fallback == "" {
		fallback = httpstate.ValueOrDefaultURL(ws, "")
	}
	url, err = session.SelectBackend(ctx, fallback, ValidateLoginBackendURL)
	return url, setCurrent, err
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
		lm := httpstate.NewLoginManager(f.session)
		credentials, err := lm.LoginWithOIDCToken(
			ctx, sink, url, insecure, authContext.Token, authContext.Organization, authContext.Scope,
			authContext.Expiration, setCurrent)
		if err != nil {
			return nil, err
		}
		f.rememberCredentials(credentials)
		return httpstate.NewWithCredentials(ctx, sink, *credentials, project)
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

	ResolveCredentialsF func(
		context.Context, pkgWorkspace.Context, string, *workspace.Project,
	) (*pkgauth.Credentials, error)

	// HelperSession supplies helper behavior for tests. Nil disables the helper.
	HelperSession *pkgauth.Session
}

var _ LoginManager = (*MockLoginManager)(nil)

func (lm *MockLoginManager) session() *pkgauth.Session {
	if lm.HelperSession != nil {
		return lm.HelperSession
	}
	return pkgauth.NewSessionWithHelperFunc(nil)
}

func (lm *MockLoginManager) ResolveCredentials(ctx context.Context, ws pkgWorkspace.Context, url string,
	project *workspace.Project,
) (*pkgauth.Credentials, error) {
	if lm.ResolveCredentialsF != nil {
		return lm.ResolveCredentialsF(ctx, ws, url, project)
	}
	return NewLoginManager(lm.session()).ResolveCredentials(ctx, ws, url, project)
}

func (lm *MockLoginManager) Login(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink, url string,
	project *workspace.Project, options LoginOptions,
) (backend.Backend, error) {
	if lm.LoginF != nil {
		url, preparedCurrent, err := prepareLogin(ctx, lm.session(), ws, url, project, options)
		options.SetCurrent = options.SetCurrent || preparedCurrent
		if err != nil {
			return nil, err
		}
		return lm.LoginF(ctx, ws, sink, url, project, options.SetCurrent, options.Insecure, options.Color)
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
