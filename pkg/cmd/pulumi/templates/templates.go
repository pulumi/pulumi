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

	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
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

	// cancel holds the function to cancel the context passed into the [New] that created the source.
	cancel context.CancelFunc
	// closers holds a list of functions to be invoked when the Source is closed.
	closers []func() error
	closed  bool

	// m should be held whenever Source is mutated.
	m  sync.Mutex
	wg sync.WaitGroup
}

// Templates lists the templates available to the [Source].
//
// Templates *does not* produce a sorted list. If templates need to be sorted, then the
// caller is responsible for sorting them.
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
	contract.Assertf(t != nil, "We should never return nil templates")
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
	contract.Assertf(!s.closed, "Attempted to act on closed source: "+action)
}

// Close cleans up the [Source] and any associated templates.
//
// Close should always be called when [Source] is dropped.
func (s *Source) Close() error {
	s.cancel()

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

type Template interface {
	Name() string
	Description() string
	ProjectDescription() string
	Error() error
	Download(ctx context.Context) (workspace.Template, error)
}

// SearchScope dictates where [New] will search for templates.
type SearchScope struct{ kind string }

var (
	// ScopeAll searches for templates in all available locations.
	ScopeAll = SearchScope{}
	// ScopeLocal searches for templates only locally (on disk).
	ScopeLocal = SearchScope{"local"}
)

// Create a new [Template] [Source] associated with a given [SearchScope].
func New(
	ctx context.Context, templateNamePathOrURL string, scope SearchScope,
	templateKind workspace.TemplateKind, e env.Env,
) *Source {
	return newImpl(
		ctx, templateNamePathOrURL, scope,
		templateKind,
		workspace.RetrieveTemplates,
		e,
	)
}

// The impl for [New].
//
// having a separate impl function allows mocking out getWorkspaceTemplates.
func newImpl(
	ctx context.Context, templateNamePathOrURL string, scope SearchScope,
	templateKind workspace.TemplateKind,
	getWorkspaceTemplates getWorkspaceTemplateFunc,
	e env.Env,
) *Source {
	var source Source
	ctx, cancel := context.WithCancel(ctx)
	source.cancel = cancel

	if scope == ScopeAll || scope == ScopeLocal {
		source.wg.Add(1)
		go func() {
			source.getWorkspaceTemplates(ctx, templateNamePathOrURL, scope, templateKind, &source.wg, getWorkspaceTemplates)
			source.wg.Done()
		}()
	}

	if scope == ScopeAll && templateKind == workspace.TemplateKindPulumiProject && isTemplateName(templateNamePathOrURL) {
		source.wg.Add(1)
		go func() {
			source.getCloudTemplates(ctx, templateNamePathOrURL, &source.wg, e)
			source.wg.Done()
		}()
	}

	return &source
}

func isTemplateName(templateNamePathOrURL string) bool {
	return !workspace.IsTemplateURL(templateNamePathOrURL) &&
		!isTemplatePath(templateNamePathOrURL)
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
