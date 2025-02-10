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
	"io/fs"
	"os"
	"strings"
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// Source provides access to a set of project templates, any set of which may be present on
// disk.
//
// Source is responsible for cleaning up old templates, and should always be [Close]d when
// created.
type Source struct {
	templates    []Template
	errorOnEmpty []error
	errors       []error
	closers      []func() error
	closed       bool

	// m should be held whenever Source is mutated.
	m  sync.Mutex
	wg sync.WaitGroup
}

// Templates lists the templates available to the [Source].
func (s *Source) Templates() ([]Template, error) {
	s.wg.Wait() // Wait to ensure that all templates have been fetched before returning the template list.

	s.lockOpen("read templates")
	defer s.m.Unlock()
	if err := errors.Join(s.errors...); err != nil {
		return nil, err
	}
	if len(s.templates) == 0 {
		return nil, errors.Join(s.errorOnEmpty...)
	}
	return s.templates, nil
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

func (s *Source) addErrorOnEmpty(err error) {
	s.lockOpen("add error on empty")
	s.errorOnEmpty = append(s.errorOnEmpty, err)
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

	queryKind := getTemplateQuery(templateNamePathOrURL)

	if scope == ScopeAll || scope == ScopeTraditional || scope == ScopeLocal {
		source.wg.Add(1)
		go func() {
			source.getWorkspaceTemplates(ctx, templateNamePathOrURL, scope, templateKind, &source.wg)
			source.wg.Done()
		}()
	}

	if scope == ScopeAll && templateKind == workspace.TemplateKindPulumiProject && queryKind == queryName {
		source.wg.Add(1)
		go func() {
			source.getOrgTemplates(ctx, templateNamePathOrURL, interactive, &source.wg)
			source.wg.Done()
		}()
	}

	return &source
}

type queryKind string

const (
	queryName queryKind = "query-name"
	queryURL  queryKind = "query-url"
	queryPath queryKind = "query-path"
)

func getTemplateQuery(query string) queryKind {
	if workspace.IsTemplateURL(query) {
		return queryURL
	}
	if isTemplatePath(query) {
		return queryPath
	}
	return queryName
}

func isTemplatePath(query string) bool {
	_, err := os.Stat(query)
	if errors.Is(err, fs.ErrNotExist) {
		if looksLikePath(query) {
			const msg = "%q looks like a file path, but no file exists. Assuming to be a template name"
			logging.Warningf(msg, query)
		}
		return false
	} else if err != nil {
		logging.Warningf("unable to stat %q: %s", query, err.Error())
		return false
	}

	// query does point to a local file.

	if !looksLikePath(query) {
		const msg = `Assuming %[1]q is a file path, use "./%[1]s" to be unambiguous`
		logging.Warningf(msg, query)
	}
	return err == nil
}

func looksLikePath(query string) bool {
	return strings.HasPrefix(query, "./") || strings.HasPrefix(query, "/")
}
