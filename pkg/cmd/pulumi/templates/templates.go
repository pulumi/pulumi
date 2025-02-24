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

// Package templates adds an abstraction for project templates that may be local or
// remote.
//
// All templates are convertible into [workspace.Template].
package templates

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// Source provides access to a set of project templates, any set of which may be present on
// disk.
//
// Source is responsible for cleaning up old templates, and should always be [Close]d when
// created.
type Source struct {
	templates []Template
	errors    []error
	closers   []func() error
	closed    bool

	// m should be held whenever Source is mutated.
	m  sync.Mutex
	wg sync.WaitGroup
}

// Templates lists the templates available to the [Source].
func (s *Source) Templates() ([]Template, error) {
	s.wg.Wait() // Wait to ensure that all templates have been fetched before returning the template list.

	s.lockOpen("read templates")
	defer s.m.Unlock()
	return s.templates, errors.Join(s.errors...)
}

func (s *Source) addTemplate(t Template) {
	s.lockOpen("add template")
	s.templates = append(s.templates, t)
	s.m.Unlock()
}

func (s *Source) addCloser(f func() error) {
	s.lockOpen("add closer")
	s.closers = append(s.closers, f)
	s.m.Unlock()
}

func (s *Source) addError(err error) {
	s.lockOpen("add error")
	s.errors = append(s.errors, err)
	s.m.Unlock()
}

func (s *Source) lockOpen(action string) {
	s.m.Lock()
	if s.closed {
		panic("Attempted to act on closed source: " + action)
	}
}

// Close cleans up the [Source] and any associated templates.
//
// Close should always be called when [Source] is dropped.
func (s *Source) Close() error {
	s.wg.Wait() // Wait to ensure that all templates have been fetched so all closers are visible.

	s.lockOpen("close")
	defer s.m.Unlock()
	s.closed = true
	errs := make([]error, len(s.closers))
	for i, f := range s.closers {
		errs[i] = f()
	}
	return errors.Join(errs...)
}

type Template struct {
	name               string
	description        string
	projectDescription string

	error error

	download func(ctx context.Context) (workspace.Template, error)

	source *Source
}

func (t Template) Name() string               { return t.name }
func (t Template) Description() string        { return t.description }
func (t Template) ProjectDescription() string { return t.projectDescription }
func (t Template) Error() error               { return t.error }

func (t Template) Download(ctx context.Context) (workspace.Template, error) {
	if t.source == nil {
		panic("Cannot download a template without a host")
	}
	if t.source.closed {
		panic("Cannot download a template from an already closed host")
	}

	return t.download(ctx)
}

type Scope struct{ kind string }

var (
	ScopeDefault     = Scope{}
	scopeDefault     = ScopeAll
	ScopeTraditional = Scope{"traditional"}
	ScopeLocal       = Scope{"none"}
	ScopeAll         = Scope{"all"}
)

// Create a new [Template] [Source] associated with a given [Scope].
func New(
	ctx context.Context, templateNamePathOrURL string, scope Scope,
	templateKind workspace.TemplateKind, interactive bool,
) *Source {
	// apply the default scope, if necessary
	if scope == ScopeDefault {
		scope = scopeDefault
	}

	var source Source
	ctx, cancel := context.WithCancel(ctx)
	source.closers = append(source.closers, func() error { cancel(); return nil })

	if scope == ScopeAll || scope == ScopeTraditional || scope == ScopeLocal {
		source.wg.Add(1)
		go func() {
			source.getWorkspaceTemplates(ctx, templateNamePathOrURL, scope, templateKind, &source.wg)
			source.wg.Done()
		}()
	}

	if scope == ScopeAll && templateKind == workspace.TemplateKindPulumiProject {
		source.wg.Add(1)
		go func() {
			source.getOrgTemplates(ctx, interactive, &source.wg)
			source.wg.Done()
		}()
	}

	return &source
}

func (s *Source) getWorkspaceTemplates(
	ctx context.Context, templateNamePathOrURL string, scope Scope, templateKind workspace.TemplateKind,
	_ *sync.WaitGroup,
) {
	repo, err := workspace.RetrieveTemplates(ctx, templateNamePathOrURL, scope == ScopeLocal, templateKind)
	if err != nil {
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
		s.addError(err)
		return
	}

	s.addDownloadedTemplates(workspaceTemplates)
}

// Retrieve a Private template from the given Pulumi Cloud URL **including an auth token for Pulumi Cloud**.
//
// workspace.GetAccount ensures the user has a valid session with the Pulumi Cloud backend.
//   - If the user is not logged in, the login flow will be initiated.
//   - If the user is not logged in and pulumi does not recognize the backend as a known workspace then
//     the user will see an authentication error.
func retrievePrivatePulumiCloudTemplate(templateURL string) (workspace.TemplateRepository, error) {
	u, err := url.Parse(templateURL)
	if err != nil {
		return workspace.TemplateRepository{}, fmt.Errorf("parsing template URL: %w", err)
	}
	// Docs convention is to store the cloud URL with the protocol.
	// e.g. `pulumi login https://api.pulumi.com` or `pulumi login https://api.acme.org`
	templatePulumiCloudHost := "https://" + u.Host

	account, err := workspace.GetAccount(templatePulumiCloudHost)
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
		s.addTemplate(Template{
			name:               t.Name,
			description:        t.Description,
			projectDescription: t.ProjectDescription,
			source:             s,
			error:              t.Error,
			download: func(context.Context) (workspace.Template, error) {
				return t, nil
			},
		})
	}
}
