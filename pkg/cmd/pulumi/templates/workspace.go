// Copyright 2016-2025, Pulumi Corporation.
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

package templates

import (
	"context"
	"errors"
	"fmt"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"net/http"
	"net/url"
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type getWorkspaceTemplateFunc = func(ctx context.Context, templateNamePathOrURL string, offline bool,
	templateKind workspace.TemplateKind,
) (workspace.TemplateRepository, error)

func (s *Source) getWorkspaceTemplates(
	ctx context.Context, templateNamePathOrURL string, scope SearchScope, templateKind workspace.TemplateKind,
	_ *sync.WaitGroup,
	get getWorkspaceTemplateFunc,
) {
	repo, err := get(ctx, templateNamePathOrURL, scope == ScopeLocal, templateKind)
	if err != nil {
		if notFound := (workspace.TemplateNotFoundError{}); errors.As(err, &notFound) {
			s.addErrorOnEmpty(notFound)
			return
		}
		// Bail on all errors unless its a 401 from a Pulumi Cloud backend...
		if !errors.Is(err, workspace.ErrPulumiCloudUnauthorized) {
			s.addError(err)
			return
		}

		// ...If the request has 401'd AND we've identified the backend as being a Pulumi Cloud instance, we can
		// attempt to retrieve the template using the user's Pulumi Cloud credentials.
		repo, err = retrievePrivatePulumiCloudTemplate(templateNamePathOrURL)
		if err != nil {
			s.addError(err)
			return
		}
	}
	s.addCloser(repo.Delete)
	workspaceTemplates, err := repo.Templates()
	if err != nil {
		s.addError(fmt.Errorf("could not get template from workspace: %w", err))
		return
	}

	s.addDownloadedTemplates(workspaceTemplates)
}

// Retrieve a Private template from the given Pulumi Cloud URL **including an auth token for Pulumi Cloud**.
func retrievePrivatePulumiCloudTemplate(templateURL string) (workspace.TemplateRepository, error) {
	u, err := url.Parse(templateURL)
	if err != nil {
		return workspace.TemplateRepository{}, fmt.Errorf("parsing template URL: %w", err)
	}
	// Docs convention is to store the cloud URL with the protocol.
	// e.g. `pulumi login https://api.pulumi.com` or `pulumi login https://api.acme.org`
	templatePulumiCloudHost := "https://" + u.Host

	// TODO: Pass Keystore or Workspace. Do not use the Singleton.
	account, err := workspace.GetAccountWithKeyStore(pkgWorkspace.Instance.GetKeyStore(), templatePulumiCloudHost)
	if err != nil {
		return workspace.TemplateRepository{}, fmt.Errorf(
			"looking up pulumi cloud backend %s: %w",
			templatePulumiCloudHost,
			err,
		)
	}

	if account.AccessToken == "" {
		return workspace.TemplateRepository{}, fmt.Errorf("no access token found for %s", templatePulumiCloudHost)
	}

	templateRepository, err := workspace.RetrieveZIPTemplates(templateURL, func(req *http.Request) {
		req.Header.Set("Authorization", "token "+account.AccessToken)
	})

	if errors.Is(err, workspace.ErrPulumiCloudUnauthorized) {
		return workspace.TemplateRepository{}, fmt.Errorf(
			"unauthorized to access template at %s. You may not have access to this template or token may have expired",
			templatePulumiCloudHost,
		)
	}

	// Caller can handle other errors
	return templateRepository, err
}

func (s *Source) addDownloadedTemplates(src []workspace.Template) {
	for _, t := range src {
		s.addTemplate(workspaceTemplate{t})
	}
}

var _ Template = workspaceTemplate{}

type workspaceTemplate struct {
	t workspace.Template
}

func (t workspaceTemplate) Name() string                                             { return t.t.Name }
func (t workspaceTemplate) Description() string                                      { return t.t.Description }
func (t workspaceTemplate) ProjectDescription() string                               { return t.t.ProjectDescription }
func (t workspaceTemplate) Error() error                                             { return t.t.Error }
func (t workspaceTemplate) Download(ctx context.Context) (workspace.Template, error) { return t.t, nil }
