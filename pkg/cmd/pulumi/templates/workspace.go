// Copyright 2016, Pulumi Corporation.
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
	"net/http"
	"net/url"

	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
)

type getProjectTemplateFunc = func(ctx context.Context, templateNamePathOrURL string, offline bool,
	templateKind TemplateKind,
) (TemplateRepository, error)

func (s *Source) getProjectTemplates(
	ctx context.Context, templateNamePathOrURL string, scope SearchScope, templateKind TemplateKind,
	get getProjectTemplateFunc,
) {
	repo, err := get(ctx, templateNamePathOrURL, scope == ScopeLocal, templateKind)
	if err != nil {
		if notFound := (TemplateNotFoundError{}); errors.As(err, &notFound) {
			s.addErrorOnEmpty(notFound)
			return
		}
		// Bail on all errors unless its a 401 from a Pulumi Cloud backend...
		if !errors.Is(err, ErrPulumiCloudUnauthorized) {
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
	projectTemplates, err := repo.Templates()
	if err != nil {
		s.addError(fmt.Errorf("could not get template from workspace: %w", err))
		return
	}

	s.addDownloadedTemplates(projectTemplates)
}

// Retrieve a Private template from the given Pulumi Cloud URL **including an auth token for Pulumi Cloud**.
func retrievePrivatePulumiCloudTemplate(templateURL string) (TemplateRepository, error) {
	u, err := url.Parse(templateURL)
	if err != nil {
		return TemplateRepository{}, fmt.Errorf("parsing template URL: %w", err)
	}
	// Docs convention is to store the cloud URL with the protocol.
	// e.g. `pulumi login https://api.pulumi.com` or `pulumi login https://api.acme.org`
	templatePulumiCloudHost := "https://" + u.Host

	account, _, err := pkgWorkspace.GetAccountWithAgentFallback(templatePulumiCloudHost)
	if err != nil {
		return TemplateRepository{}, fmt.Errorf(
			"looking up pulumi cloud backend %s: %w",
			templatePulumiCloudHost,
			err,
		)
	}

	if account.AccessToken == "" {
		return TemplateRepository{}, fmt.Errorf("no access token found for %s", templatePulumiCloudHost)
	}

	templateRepository, err := RetrieveZIPTemplates(templateURL, func(req *http.Request) {
		req.Header.Set("Authorization", "token "+account.AccessToken)
	})

	if errors.Is(err, ErrPulumiCloudUnauthorized) {
		return TemplateRepository{}, fmt.Errorf(
			"unauthorized to access template at %s. You may not have access to this template or token may have expired",
			templatePulumiCloudHost,
		)
	}

	// Caller can handle other errors
	return templateRepository, err
}

func (s *Source) addDownloadedTemplates(src []ProjectTemplate) {
	for _, t := range src {
		s.addTemplate(projectTemplate{t})
	}
}

var _ Template = projectTemplate{}

type projectTemplate struct {
	t ProjectTemplate
}

func (t projectTemplate) Name() string        { return t.t.Name }
func (t projectTemplate) DisplayName() string { return t.t.Name }
func (t projectTemplate) Description() string { return t.t.Description }
func (t projectTemplate) Error() error        { return t.t.Error }
func (t projectTemplate) Download(ctx context.Context) (ProjectTemplate, error) {
	return t.t, nil
}
