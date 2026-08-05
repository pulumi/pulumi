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

// fetch is one independent template fetch. Each fetch owns everything it produced — templates,
// errors, and whatever the listing reported on the side — so that a caller can join one fetch
// without waiting on the others, and so that no two fetches share a field to race over.
type fetch struct {
	wg sync.WaitGroup

	// m guards everything below: a fetch may fan out into goroutines of its own that report
	// results concurrently.
	m         sync.Mutex
	templates []Template
	errs      []error

	// errsOnEmpty are reported only if the [Source] as a whole found nothing. A fetch that came
	// up empty is not itself an error; another may still have found something.
	errsOnEmpty []error

	// vcsOrgs names organizations the service reported as having VCS-backed template
	// collections. Only a listing that asks the service for them fills this in.
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

// join waits for the fetch to finish, then returns its templates, or its errors if it produced
// any.
func (f *fetch) join() ([]Template, error) {
	f.wg.Wait()

	f.m.Lock()
	defer f.m.Unlock()
	if err := errors.Join(f.errs...); err != nil {
		return nil, err
	}
	return slices.Clone(f.templates), nil
}

// joinVcsOrgs waits for the fetch to finish, then returns the organizations it observed.
func (f *fetch) joinVcsOrgs() []string {
	f.wg.Wait()

	f.m.Lock()
	defer f.m.Unlock()
	return slices.Clone(f.vcsOrgs)
}

// cleanup collects the deletions a [Source] owes when it closes. Templates hold this rather than
// the whole Source: downloading one registers a temp directory, which should not also hand it
// access to the fetches.
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

// assertOpen panics if the source has already been closed, which would mean the templates it
// handed out have been deleted from disk.
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
	// The fetches, fastest first. They are speed tiers, not categories: project is the local disk
	// and curated template set, database is what the service answers from its own tables without
	// leaving the building, and upstream is what it must fetch from a version control provider on
	// every request. Tracking them apart lets a caller join one without waiting on the rest.
	//
	// Each fetch owns its own results, so Source holds no state they could race over; it only
	// composes them.
	project  fetch
	database fetch
	upstream fetch

	// cleanup is shared with the templates the fetches hand out, so a downloaded template can
	// register its temp directory for deletion.
	cleanup cleanup

	// cancel holds the function to cancel the context passed into the [New] that created the source.
	cancel context.CancelFunc
}

// fetches lists the fetches, fastest first. Anything that spans all of them — waiting, joining
// errors — should range over this so a new fetch only needs to be added here.
func (s *Source) fetches() []*fetch {
	return []*fetch{&s.project, &s.database, &s.upstream}
}

// waitAll blocks until every fetch has finished.
func (s *Source) waitAll() {
	for _, w := range s.fetches() {
		w.wg.Wait()
	}
}

// Templates lists the templates available to the [Source].
//
// Templates *does not* produce a sorted list; it groups the fetches in the order they are declared
// on [Source]. If templates need to be sorted, then the caller is responsible for sorting them.
func (s *Source) Templates() ([]Template, error) {
	// Wait to ensure that all templates have been fetched before returning the template list.
	s.waitAll()
	s.cleanup.assertOpen("read templates")

	var errs []error
	for _, w := range s.fetches() {
		errs = append(errs, w.errs...)
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}

	// A registry template already produced by an earlier fetch is dropped. A service that does
	// not honor the backing filter answers each fetch with everything it has, which would
	// otherwise list every cloud template twice.
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

// registryIdentity is the source/publisher/name triple that names a template in the registry.
func registryIdentity(t TemplateMatchable) string {
	return t.GetSource() + "/" + t.GetPublisher() + "/" + t.GetRegistryName()
}

// ProjectTemplates lists only the templates found by the project fetcher (local disk and the
// curated template set), without waiting for the slower cloud fetches, which keep running in the
// background. Use [Source.Templates] for the complete set.
//
// Unlike [Source.Templates], an empty result is not an error here: a fetch that is still running
// may yet produce templates, so only the complete set can conclude that there are none.
func (s *Source) ProjectTemplates() ([]Template, error) {
	return s.project.join()
}

// DatabaseTemplates lists the cloud templates the service answers from its own tables, without
// waiting on the collections it would have to fetch upstream. As with [Source.ProjectTemplates],
// an empty result is not an error.
//
// Only a [Source] created to browse — one given no template name — splits the cloud listing far
// enough for this tier to hold anything; every other path runs one unfiltered listing as the
// upstream fetch and answers here with nothing. Callers that need the complete set must use
// [Source.Templates] and pay the upstream cost.
func (s *Source) DatabaseTemplates() ([]Template, error) {
	return s.database.join()
}

// VcsTemplateSourceOrgs names organizations that have VCS-backed template collections configured.
// Their templates are only in [Source.Templates], never in [Source.DatabaseTemplates].
//
// The result is best-effort in both directions: an organization listed here has at least one
// collection, but one absent from here may still have them — because the service did not report
// it, or because this [Source] did not run the listing that reports it.
// Every listing records what the service reported, but only the database fetch's answer is
// read, so the two cloud listings never contend and the result does not depend on which of
// them finished first.
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
	// Publisher returns the organization that published this template to the registry. It is
	// empty for templates without one, such as the curated pulumi/templates set.
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
			// Use the old org templates based API.
			//
			// This path can be removed when we are confident in registry resolution. We will
			// always need to maintain a way to access templates without the service, but we
			// should only need to maintain one way to access templates through the service.
			//
			// It has no notion of a backing and fetches every org's collections upstream, so it
			// runs wholly as the upstream fetch.
			source.upstream.wg.Go(func() {
				source.upstream.listOrgTemplates(ctx, templateNamePathOrURL, e, &source.cleanup)
			})
		case templateNamePathOrURL == "":
			// Browsing splits the cloud listing in two so that the database half can be joined
			// without waiting on the upstream half. Neither asks for
			// [apitype.TemplateBackingPulumi]: those are the curated templates, which the
			// project fetch already has from its own checkout.
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
			// Resolving a name has no prompt to render early, so it takes one unfiltered
			// lookup rather than paying for two. That lookup pays the upstream fan-out, so it
			// runs wholly as the upstream fetch.
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
