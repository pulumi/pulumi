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

// Package templates adds an abstraction for project templates that may be local or
// remote.
//
// All templates are convertible into [ProjectTemplate].
package templates

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/pulumi/pulumi/pkg/v3/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type fetch struct {
	wg sync.WaitGroup

	m         sync.Mutex
	templates []Template
	errs      []error

	// errsOnEmpty are reported only if the [Source] as a whole found nothing.
	errsOnEmpty []error

	vcsOrgs []string
}

func (f *fetch) addTemplate(t Template) {
	contract.Assertf(t != nil, "We should never return nil templates")
	f.m.Lock()
	f.templates = append(f.templates, t)
	f.m.Unlock()
}

func (f *fetch) addError(err error) {
	f.m.Lock()
	f.errs = append(f.errs, err)
	f.m.Unlock()
}

func (f *fetch) addErrorOnEmpty(err error) {
	f.m.Lock()
	f.errsOnEmpty = append(f.errsOnEmpty, err)
	f.m.Unlock()
}

func (f *fetch) addVcsOrgs(totals []apitype.OrgVcsTemplateSourceTotal) {
	f.m.Lock()
	defer f.m.Unlock()
	for _, total := range totals {
		if !slices.Contains(f.vcsOrgs, total.OrgLogin) {
			f.vcsOrgs = append(f.vcsOrgs, total.OrgLogin)
		}
	}
}

func (f *fetch) join() ([]Template, error) {
	f.wg.Wait()

	f.m.Lock()
	defer f.m.Unlock()
	if err := errors.Join(f.errs...); err != nil {
		return nil, err
	}
	return slices.Clone(f.templates), nil
}

func (f *fetch) joinVcsOrgs() []string {
	f.wg.Wait()

	f.m.Lock()
	defer f.m.Unlock()
	return slices.Clone(f.vcsOrgs)
}

type cleanup struct {
	m       sync.Mutex
	closed  bool
	closers []func() error
}

func (c *cleanup) add(f func() error) {
	c.m.Lock()
	defer c.m.Unlock()
	contract.Assertf(!c.closed, "Attempted to add a closer to a closed source")
	c.closers = append(c.closers, f)
}

func (c *cleanup) assertOpen(action string) {
	c.m.Lock()
	defer c.m.Unlock()
	contract.Assertf(!c.closed, "%s", "Attempted to act on closed source: "+action)
}

func (c *cleanup) run() error {
	c.m.Lock()
	defer c.m.Unlock()
	c.closed = true
	errs := make([]error, len(c.closers))
	for i, f := range c.closers {
		errs[i] = f()
	}
	return errors.Join(errs...)
}

// Source provides access to a set of project templates, any set of which may be present on
// disk.
//
// Source is responsible for cleaning up old templates, and should always be [Close]d when
// created.
type Source struct {
	project  fetch
	database fetch
	upstream fetch

	cleanup cleanup

	// cancel holds the function to cancel the context passed into the [New] that created the source.
	cancel context.CancelFunc
}

func (s *Source) fetches() []*fetch {
	return []*fetch{&s.project, &s.database, &s.upstream}
}

func (s *Source) waitAll() {
	for _, w := range s.fetches() {
		w.wg.Wait()
	}
}

// Templates lists the templates available to the [Source].
//
// Templates *does not* produce a sorted list. If templates need to be sorted, then the
// caller is responsible for sorting them.
func (s *Source) Templates() ([]Template, error) {
	s.waitAll()
	s.cleanup.assertOpen("read templates")

	var errs []error
	for _, w := range s.fetches() {
		errs = append(errs, w.errs...)
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}

	// A service that does not honor the backing filter answers each fetch with everything it has.
	seen := map[string]bool{}
	var all []Template
	for _, w := range s.fetches() {
		for _, t := range w.templates {
			if m, ok := t.(TemplateMatchable); ok {
				id := registryIdentity(m)
				if seen[id] {
					continue
				}
				seen[id] = true
			}
			all = append(all, t)
		}
	}
	if len(all) == 0 {
		var onEmpty []error
		for _, w := range s.fetches() {
			onEmpty = append(onEmpty, w.errsOnEmpty...)
		}
		return nil, errors.Join(onEmpty...)
	}
	return all, nil
}

func registryIdentity(t TemplateMatchable) string {
	return t.GetSource() + "/" + t.GetPublisher() + "/" + t.GetRegistryName()
}

func (s *Source) ProjectTemplates() ([]Template, error) {
	return s.project.join()
}

func (s *Source) DatabaseTemplates() ([]Template, error) {
	return s.database.join()
}

func (s *Source) VcsTemplateSourceOrgs() []string {
	return s.database.joinVcsOrgs()
}

// Close cleans up the [Source] and any associated templates.
//
// Close should always be called when [Source] is dropped.
func (s *Source) Close() error {
	s.cancel()

	// Wait to ensure that all templates have been fetched so all closers are visible.
	s.waitAll()

	return s.cleanup.run()
}

// A template entry to show in the chooser.
type Template interface {
	Name() string
	DisplayName() string
	Description() string
	Error() error
	Publisher() string
	// Download the template and return an instantiable [ProjectTemplate] for this template.
	Download(ctx context.Context) (ProjectTemplate, error)
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
	templateKind TemplateKind, e env.Env,
) *Source {
	return newImpl(
		ctx, templateNamePathOrURL, scope,
		templateKind,
		RetrieveTemplates,
		e,
	)
}

// The impl for [New].
//
// having a separate impl function allows mocking out getProjectTemplates.
func newImpl(
	ctx context.Context, templateNamePathOrURL string, scope SearchScope,
	templateKind TemplateKind,
	getProjectTemplates getProjectTemplateFunc,
	e env.Env,
) *Source {
	var source Source
	ctx, cancel := context.WithCancel(ctx)
	source.cancel = cancel

	if scope == ScopeAll || scope == ScopeLocal {
		source.project.wg.Go(func() {
			source.project.listProjectTemplates(
				ctx, templateNamePathOrURL, scope, templateKind, getProjectTemplates, &source.cleanup,
			)
		})
	}

	if scope == ScopeAll && templateKind == TemplateKindPulumiProject && isTemplateName(templateNamePathOrURL) {
		switch {
		case e.GetBool(env.DisableRegistryResolve):
			// TODO[pulumi/pulumi#24250]: remove the org templates API once we're confident in
			// registry resolution.
			source.upstream.wg.Go(func() {
				source.upstream.listOrgTemplates(ctx, templateNamePathOrURL, e, &source.cleanup)
			})
		case templateNamePathOrURL == "":
			// Split so the database half can be joined without waiting on the upstream half.
			// Neither asks for [apitype.TemplateBackingPulumi]: the project fetch has those.
			r := defaultRegistry(ctx, e)
			source.database.wg.Go(func() {
				source.database.listRegistry(ctx, r, registry.ListTemplatesOptions{
					Backing: []apitype.TemplateBacking{apitype.TemplateBackingRegistry},
				}, &source.cleanup)
			})
			source.upstream.wg.Go(func() {
				source.upstream.listRegistry(ctx, r, registry.ListTemplatesOptions{
					Backing: []apitype.TemplateBacking{apitype.TemplateBackingVcs},
				}, &source.cleanup)
			})
		default:
			source.upstream.wg.Go(func() {
				source.upstream.resolveRegistryName(
					ctx, defaultRegistry(ctx, e), templateNamePathOrURL, &source.cleanup,
				)
			})
		}
	}

	return &source
}

func isTemplateName(templateNamePathOrURL string) bool {
	return !IsGitRepoTemplateURL(templateNamePathOrURL) &&
		!isTemplatePath(templateNamePathOrURL)
}

func isTemplatePath(query string) bool {
	_, err := os.Stat(query)
	if errors.Is(err, fs.ErrNotExist) {
		if looksLikePath(query) {
			const msg = "%q looks like a file path, but no file exists. Assuming to be a template name"
			slog.Warn(fmt.Sprintf(msg, query))
		}
		return false
	} else if err != nil {
		slog.Warn("unable to stat", "query", query, "err", err.Error())
		return false
	}

	// query does point to a local file.

	if !looksLikePath(query) {
		const msg = `Assuming %[1]q is a file path, use "./%[1]s" to be unambiguous`
		slog.Warn(fmt.Sprintf(msg, query))
	}
	return err == nil
}

func looksLikePath(query string) bool {
	return strings.HasPrefix(query, "./") || strings.HasPrefix(query, "/")
}
