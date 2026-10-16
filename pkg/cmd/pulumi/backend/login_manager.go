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
	"github.com/pulumi/pulumi/pkg/v3/auth/credentialhelper"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/diy"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	backendsecrets "github.com/pulumi/pulumi/pkg/v3/backend/secrets"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// LoginManager provides a slim wrapper around functions related to backend logins.
type LoginManager interface {
	// CredentialHelperSession owns helper discovery and backend preparations for this command.
	CredentialHelperSession() *credentialhelper.Session
	// ClientSource shares authenticated clients for this command's backends and secrets providers.
	ClientSource() *httpstate.ClientSource
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

var DefaultLoginManager = NewLoginManager(credentialhelper.NewSession())

type lm struct {
	session *credentialhelper.Session
	clients *httpstate.ClientSource
}

// NewLoginManager shares backend clients and helper credentials for one command.
func NewLoginManager(session *credentialhelper.Session) LoginManager {
	if session == nil {
		session = credentialhelper.NewSession()
	}
	return &lm{session: session, clients: httpstate.NewClientSource(session, cmdutil.Diag())}
}

// ClientSource returns the command's authenticated backend clients.
func (f *lm) ClientSource() *httpstate.ClientSource { return f.clients }

// SecretsProvider constructs standard secrets managers using the login manager's clients.
func SecretsProvider(lm LoginManager) secrets.Provider {
	return backendsecrets.NewProvider(lm.ClientSource().ClientForURL)
}

// CredentialHelperSession returns the helper session explicitly owned by this manager.
func (f *lm) CredentialHelperSession() *credentialhelper.Session {
	return f.session
}

func (f *lm) Current(
	ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink, url string, project *workspace.Project, setCurrent bool,
) (backend.Backend, error) {
	var err error
	session := f.CredentialHelperSession()
	url, setCurrent, err = prepareBackend(ctx, session, ws, url, setCurrent)
	if err != nil {
		return nil, err
	}
	if diy.IsDIYBackendURL(url) {
		be, err := diy.New(ctx, sink, url, project)
		if err == nil && setCurrent {
			err = credentialhelper.SetCurrent(url)
		}
		return be, err
	}

	insecure := pkgWorkspace.GetCloudInsecure(ws, url)
	lm := httpstate.NewLoginManagerWithCredentials(session.Lookup(url))
	account, err := lm.Current(ctx, url, insecure, setCurrent)
	if err != nil || account == nil {
		return nil, err
	}
	return f.openHTTPBackend(ctx, sink, url, project, insecure, setCurrent, session.Lookup(url))
}

func (f *lm) Login(
	ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink, url string, project *workspace.Project, setCurrent bool,
	insecure bool, color colors.Colorization,
) (backend.Backend, error) {
	var err error
	session := f.CredentialHelperSession()
	url, setCurrent, err = prepareBackend(ctx, session, ws, url, setCurrent)
	if err != nil {
		return nil, err
	}
	if diy.IsDIYBackendURL(url) {
		if setCurrent {
			return diy.Login(ctx, sink, url, project)
		}
		return diy.New(ctx, sink, url, project)
	}

	lm := httpstate.NewLoginManagerWithCredentials(session.Lookup(url))
	// Color is the only thing used by lm.Login, so we can just request a colors.Colorization and only fill that part of
	// the display options in. It's hard to change Login itself because it's circularly depended on by esc.
	opts := display.Options{
		Color: color,
	}
	consoleURL := client.CloudConsoleURL(httpstate.ValueOrDefaultURL(ws, url))
	welcome := func(opts display.Options) { httpstate.WelcomeUser(opts, consoleURL) }
	_, err = lm.Login(ctx, url, insecure, "pulumi", "Pulumi stacks", welcome, setCurrent, opts)
	if err != nil {
		return nil, err
	}
	return f.openHTTPBackend(ctx, sink, url, project, insecure, setCurrent, session.Lookup(url))
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
		lm := httpstate.NewLoginManagerWithCredentials(nil) // OIDC login doesn't use credentials helpers
		_, err := lm.LoginWithOIDCToken(
			ctx, sink, url, insecure, authContext.Token, authContext.Organization, authContext.Scope,
			authContext.Expiration, setCurrent)
		if err != nil {
			return nil, err
		}
		return f.openHTTPBackend(ctx, sink, url, project, insecure, setCurrent, nil)
	}
	return nil, fmt.Errorf("unknown auth context grant type: %s", authContext.GrantType)
}

type MockLoginManager struct {
	Clients  *httpstate.ClientSource
	Helpers  *credentialhelper.Session
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
}

var _ LoginManager = (*MockLoginManager)(nil)

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

func prepareBackend(
	ctx context.Context, s *credentialhelper.Session, ws pkgWorkspace.Context, raw string, setCurrent bool,
) (string, bool, error) {
	if raw == "" {
		raw = env.APIURL.Value()
	}
	r, err := s.Resolve(ctx, raw)
	if err != nil {
		return "", false, err
	}
	resolved := r.BackendURL()
	selectedByHelper := r.SelectedBackend()
	if resolved == "" {
		resolved = httpstate.ValueOrDefaultURL(ws, "")
	}
	if err := s.SelectBackend(r, resolved); err != nil {
		return "", false, err
	}
	return resolved, setCurrent || selectedByHelper, nil
}

func (f *lm) openHTTPBackend(
	ctx context.Context, sink diag.Sink, raw string, project *workspace.Project,
	insecure, setCurrent bool, helper *credentialhelper.Result,
) (backend.Backend, error) {
	be, err := httpstate.New(ctx, sink, raw, project, insecure, f.clients)
	if err != nil {
		return nil, err
	}
	if setCurrent && helper != nil && helper.HasHTTP() {
		account, err := workspace.GetAccount(raw)
		if err != nil {
			return nil, err
		}
		// Persist the connection setting while preserving stored credentials
		account.Insecure = insecure
		if err := workspace.StoreAccount(raw, account, true); err != nil {
			return nil, err
		}
	}
	return be, nil
}

// CredentialHelperSession returns the helper session used by a test login manager.
func (m *MockLoginManager) CredentialHelperSession() *credentialhelper.Session {
	if m.Helpers == nil {
		m.Helpers = credentialhelper.NewSession()
	}
	return m.Helpers
}

func (m *MockLoginManager) ClientSource() *httpstate.ClientSource {
	if m.Clients == nil {
		m.Clients = httpstate.NewClientSource(m.CredentialHelperSession(), cmdutil.Diag())
	}
	return m.Clients
}
