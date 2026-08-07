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
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/registry"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type TemplateMatchable interface {
	GetRegistryName() string
	GetTemplateName() string
	GetSource() string
	GetPublisher() string
}

// parseTemplateURL parses a template name/URL into a URLInfo structure.
// Returns nil URLInfo (not an error) for plain names that should fall back to name matching.
func parseTemplateURL(templateName string) (*registry.URLInfo, error) {
	if templateName == "" {
		return nil, nil
	}

	// 1. Try parsing as a strict registry:// URL
	if registry.IsRegistryURL(templateName) {
		urlInfo, err := registry.ParseRegistryURL(templateName)
		if err != nil {
			var invalidRegistryURL *registry.InvalidRegistryURLError
			if errors.As(err, &invalidRegistryURL) {
				// Wrap this particular error reason because formats other than the
				// full registry:// URL format are supported by `pulumi new`.
				if strings.Contains(invalidRegistryURL.Reason, "expected format") {
					return nil, errors.New("Expected: registry://templates/source/publisher/name[@version], " +
						"source/publisher/name[@version], publisher/name[@version], or name[@version]")
				}
			}
			return nil, err
		}
		if urlInfo.ResourceType() != "templates" {
			return nil, fmt.Errorf("resource type '%s' is not valid for templates", urlInfo.ResourceType())
		}
		return urlInfo, nil
	}

	// 2. Try parsing as a partial registry URL
	urlInfo, err := registry.ParsePartialRegistryURL(templateName, "templates")
	if err != nil {
		var missingVersion *registry.MissingVersionAfterAtSignError
		if errors.As(err, &missingVersion) {
			return nil, err
		}

		// Structural errors: fall back to name matching
		return nil, nil
	}
	return urlInfo, nil
}

// NewTemplateMatcher creates a matcher function from parsed URL info.
func NewTemplateMatcher(urlInfo *registry.URLInfo, templateName string) func(TemplateMatchable) bool {
	// Empty template name matches everything
	if templateName == "" {
		return func(TemplateMatchable) bool { return true }
	}

	if urlInfo == nil {
		return func(t TemplateMatchable) bool {
			return t.GetRegistryName() == templateName || t.GetTemplateName() == templateName
		}
	}

	return func(t TemplateMatchable) bool {
		if urlInfo.Source() != "" && t.GetSource() != urlInfo.Source() {
			return false
		}
		if urlInfo.Publisher() != "" && t.GetPublisher() != urlInfo.Publisher() {
			return false
		}
		if urlInfo.Name() != "" {
			return t.GetRegistryName() == urlInfo.Name() || t.GetTemplateName() == urlInfo.Name()
		}
		return true
	}
}

// defaultRegistry is the registry the cloud fetches share. It resolves its backend lazily and
// only once, so fetches that list concurrently pay for a single backend lookup.
func defaultRegistry(ctx context.Context, e env.Env) registry.Registry {
	return cmdCmd.NewDefaultRegistry(ctx, cmdBackend.DefaultLoginManager, pkgWorkspace.Instance, nil, cmdutil.Diag(), e)
}

// listRegistry runs one registry listing with the options its scheduler chose; which fetch asks
// for what lives in [newImpl].
func (f *fetch) listRegistry(
	ctx context.Context, r registry.Registry, opts registry.ListTemplatesOptions, c *cleanup,
) {
	f.listRegistryMatching(ctx, r, opts, func(TemplateMatchable) bool { return true }, c)
}

// resolveRegistryName resolves a template name, URL, or partial URL against the registry. A name
// pinned to a version is looked up directly; anything else runs an unfiltered listing and keeps
// the matches.
func (f *fetch) resolveRegistryName(
	ctx context.Context, r registry.Registry, templateName string, c *cleanup,
) {
	urlInfo, err := parseTemplateURL(templateName)
	if err != nil {
		f.addError(err)
		return
	}
	if urlInfo != nil && urlInfo.Version() != nil {
		f.resolveRegistryVersion(ctx, r, urlInfo, c)
		return
	}
	f.listRegistryMatching(ctx, r, registry.ListTemplatesOptions{}, NewTemplateMatcher(urlInfo, templateName), c)
}

func (f *fetch) listRegistryMatching(
	ctx context.Context, r registry.Registry, opts registry.ListTemplatesOptions,
	matches func(TemplateMatchable) bool, c *cleanup,
) {
	for page, err := range r.ListTemplates(ctx, opts) {
		if err != nil {
			f.addError(fmt.Errorf("could not get template: %w", err))
			return
		}

		// The totals describe every organization the caller belongs to, not the page, so every
		// page carries the same answer. Recording into this fetch keeps listings from contending.
		f.addVcsOrgs(page.VcsTemplateSourceTotals)

		for _, template := range page.Templates {
			if template.Source == "github" && strings.HasPrefix(template.Name, "pulumi/templates/") {
				// These templates are maintained using https://github.com/pulumi/templates, and are
				// ingested without going through the Pulumi Cloud.
				continue
			}

			t := registryTemplate{template, r, c}
			if !matches(t) {
				continue
			}

			f.addTemplate(t)
		}
	}
}

func (f *fetch) resolveRegistryVersion(
	ctx context.Context,
	r registry.Registry,
	urlInfo *registry.URLInfo,
	c *cleanup,
) {
	version := urlInfo.Version()
	displayName := buildResolveName(urlInfo)

	var template apitype.TemplateMetadata
	var err error
	if urlInfo.Source() != "" && urlInfo.Publisher() != "" {
		// Use direct GetTemplate to preserve names containing '/' (VCS paths),
		// which ResolveTemplateFromName would split on.
		template, err = r.GetTemplate(ctx, urlInfo.Source(), urlInfo.Publisher(), urlInfo.Name(), version)
	} else {
		template, err = registry.ResolveTemplateFromName(ctx, r, displayName, version)
	}

	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			f.addError(fmt.Errorf("template '%s' version '%s' not found",
				displayName, version.String()))
			return
		}
		f.addError(fmt.Errorf("could not resolve template: %w", err))
		return
	}

	if template.Source == "github" && strings.HasPrefix(template.Name, "pulumi/templates/") {
		f.addError(fmt.Errorf(
			"template '%s' is VCS-backed and does not support specific versions",
			displayName,
		))
		return
	}

	f.addTemplate(registryTemplate{template, r, c})
}

func buildResolveName(u *registry.URLInfo) string {
	parts := make([]string, 0, 3)
	if u.Source() != "" {
		parts = append(parts, u.Source())
	}
	if u.Publisher() != "" {
		parts = append(parts, u.Publisher())
	}
	parts = append(parts, u.Name())
	return strings.Join(parts, "/")
}

type registryTemplate struct {
	t        apitype.TemplateMetadata
	registry registry.Registry
	cleanup  *cleanup
}

var _ Template = registryTemplate{}

func (r registryTemplate) Name() string {
	switch r.t.Source {
	case "github", "gitlab":
		parts := strings.SplitN(r.t.Name, "/", 3)
		return parts[len(parts)-1]
	default:
		return r.t.Name
	}
}

func (r registryTemplate) DisplayName() string {
	// To help with disambiguation we show the "origin" of templates. For VCS backed templates we show the repo slug.
	// For registry backed templates we show the publisher (= organisation name).
	//
	// Note that the default templates from https://github.com/pulumi/templates are not included here, they are not
	// `registryTemplate` instances, so these are shown without extra annotation.
	switch r.t.Source {
	case "github", "gitlab":
		nameParts := strings.SplitN(r.t.Name, "/", 3)
		name := nameParts[len(nameParts)-1]
		if r.t.RepoSlug != nil {
			return fmt.Sprintf("%s [%s]", name, *r.t.RepoSlug)
		}
		return name
	default:
		if r.GetPublisher() != "" {
			return fmt.Sprintf("%s [%s]", r.t.Name, r.GetPublisher())
		}
		return r.t.Name
	}
}

func (r registryTemplate) Description() string {
	if r.t.Description != nil {
		return *r.t.Description
	}
	return ""
}

func (r registryTemplate) Error() error { return nil }

func (r registryTemplate) Publisher() string { return r.GetPublisher() }

func (r registryTemplate) Download(ctx context.Context) (ProjectTemplate, error) {
	templateBytes, err := r.registry.DownloadTemplate(ctx, r.t.DownloadURL)
	if err != nil {
		return ProjectTemplate{}, fmt.Errorf("failed to download from %q: %w", r.t.DownloadURL, err)
	}
	defer contract.IgnoreClose(templateBytes)
	templateDir, err := os.MkdirTemp("", "pulumi-template-")
	if err != nil {
		return ProjectTemplate{}, fmt.Errorf("failed to make temporary directory: %w", err)
	}
	// Having created a template directory, we now add it to the list of directories to close.
	r.cleanup.add(func() error { return os.RemoveAll(templateDir) })
	tarReader, err := createTarReader(templateBytes)
	if err != nil {
		return ProjectTemplate{}, fmt.Errorf("failed to create tar reader: %w", err)
	}
	defer tarReader.Close()

	if err := writeTar(ctx, tar.NewReader(tarReader), templateDir); err != nil {
		return ProjectTemplate{}, err
	}

	template, err := LoadTemplate(templateDir)
	return template, err
}

func (r registryTemplate) GetRegistryName() string { return r.t.Name }
func (r registryTemplate) GetTemplateName() string { return r.Name() }
func (r registryTemplate) GetSource() string       { return r.t.Source }
func (r registryTemplate) GetPublisher() string    { return r.t.Publisher }

func (f *fetch) listOrgTemplates(ctx context.Context, templateName string, e env.Env, c *cleanup) {
	cwd, err := os.Getwd()
	if err != nil {
		f.addError(fmt.Errorf("getting current working directory: %w", err))
		return
	}

	ws := pkgWorkspace.Instance
	project, _, err := ws.ReadProject(cwd)
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		f.addError(fmt.Errorf("could not read the current project: %w", err))
		return
	}

	url, err := pkgWorkspace.GetCurrentCloudURL(ws, e, project)
	if err != nil {
		f.addError(fmt.Errorf("could not get current cloud url: %w", err))
		return
	}

	b, err := cmdBackend.DefaultLoginManager.Current(ctx, ws, cmdutil.Diag(), url, project, false)
	if err != nil {
		if !errors.Is(err, backenderr.MissingEnvVarForNonInteractiveError{}) {
			f.addError(fmt.Errorf("could not get the current backend: %w", err))
		}
		slog.InfoContext(ctx, "could not get a backend for org templates")
		return
	} else if b == nil {
		slog.InfoContext(ctx, "no current logged in user")
		return
	}

	// Attempt to retrieve the current user
	if _, _, _, err := b.CurrentUser(); err != nil {
		if errors.Is(err, backenderr.ErrLoginRequired) {
			slog.InfoContext(ctx, "user is not logged in")
			return // No current user - so don't proceed
		}
		f.addError(fmt.Errorf("could not get the current user for %s: %s", url, err))
		return
	}

	if !b.SupportsTemplates() {
		slog.InfoContext(ctx, "does not support Org Templates", "backend", b.Name())
		return
	}

	slog.InfoContext(ctx, "Listing Org Templates from the cloud")
	user, orgs, _, err := b.CurrentUser()
	if err != nil {
		f.addError(fmt.Errorf("could not get the current user: %w", err))
		return
	} else if user == "" {
		return // No current user - so don't proceed.
	}

	alreadySeenSourceURLs := map[string]struct{}{}

	handleOrg := func(org string) {
		slog.InfoContext(ctx, "Checking for templates", "org", org)
		orgTemplates, err := b.ListTemplates(ctx, org)
		if apiError := new(apitype.ErrorResponse); errors.As(err, &apiError) {
			// This is what happens when we try to access org templates for an org that hasn't enabled org templates.
			if apiError.Code == 402 {
				slog.InfoContext(ctx, "does not have access to org templates", "org", org, "code", apiError.Code)
				return
			}
		} else if err != nil {
			f.addError(fmt.Errorf("list templates: %w", err))
			slog.WarnContext(ctx, "Failed to get templates", "org", org, "err", err.Error())
			return
		} else if orgTemplates.HasAccessError {
			slog.WarnContext(ctx,
				"Failed to get templates: access denied; check that the backend can access all template sources",
				"org", org, "backend", b.Name())
			return
		} else if orgTemplates.HasUpstreamError {
			// This is a catch-all error indicating only that *something* went
			// wrong with fetching templates for an org.
			slog.WarnContext(ctx, "Failed to get templates: the backend could not download the template",
				"org", org, "backend", b.Name())
			return
		}

		for source, sourceTemplates := range orgTemplates.Templates {
			slog.InfoContext(ctx, "sourcing templates", "source", source)
			for _, template := range sourceTemplates {
				// These template are maintained using https://github.com/pulumi/templates, and are
				// ingested without going through the Pulumi Cloud.
				//
				//
				if strings.HasPrefix(template.TemplateURL, "https://github.com/pulumi/templates") {
					continue
				}

				// Check if we already have this template from another source.
				if _, ok := alreadySeenSourceURLs[template.TemplateURL]; ok {
					// Skip a template that we have already seen.
					continue
				}
				alreadySeenSourceURLs[template.TemplateURL] = struct{}{}

				// If we are searching for a template of a specific name,
				// only match templates of that name.
				if templateName != "" && templateName != template.Name {
					slog.DebugContext(ctx, "skipping template", "template", template.Name)
					continue
				}

				slog.DebugContext(ctx, "adding template", "template", template.Name)
				f.addTemplate(orgTemplate{
					t:       template,
					org:     org,
					cleanup: c,
					backend: b,
				})
			}
		}
	}

	for _, org := range orgs {
		f.wg.Go(func() { handleOrg(org) })
	}
}

type orgTemplate struct {
	t       *apitype.PulumiTemplateRemote
	org     string
	cleanup *cleanup
	backend backend.Backend
}

var _ Template = (*orgTemplate)(nil)

func (t orgTemplate) Name() string        { return t.t.Name }
func (t orgTemplate) DisplayName() string { return t.t.Name }
func (t orgTemplate) Description() string { return t.t.Description }
func (t orgTemplate) Error() error        { return nil }
func (t orgTemplate) Publisher() string   { return t.org }
func (t orgTemplate) Download(ctx context.Context) (ProjectTemplate, error) {
	templateDir, err := os.MkdirTemp("", "pulumi-template-")
	if err != nil {
		return ProjectTemplate{}, err
	}
	// Having created a template directory, we now add it to the list of directories to close.
	t.cleanup.add(func() error { return os.RemoveAll(templateDir) })

	tarReader, err := t.backend.DownloadTemplate(ctx, t.org, t.t.TemplateURL)
	if err != nil {
		return ProjectTemplate{}, err
	}
	if err := errors.Join(
		writeTar(ctx, tarReader.Tar(), templateDir),
		tarReader.Close(),
	); err != nil {
		return ProjectTemplate{}, err
	}
	slog.InfoContext(ctx, "downloaded template", "template", t.t.Name, "dir", templateDir)

	return LoadTemplate(templateDir)
}

const maxDecompressedSize = 100 << 20 // 100MB

// isGzipMagic checks if the given bytes start with the gzip magic number.
// See https://datatracker.ietf.org/doc/html/rfc1952#section-2
func isGzipMagic(header []byte) bool {
	return len(header) >= 2 && header[0] == 0x1f && header[1] == 0x8b
}

func createTarReader(reader io.Reader) (io.ReadCloser, error) {
	peekReader := bufio.NewReader(reader)
	header, err := peekReader.Peek(2)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("failed to peek at template stream: %w", err)
	}

	if isGzipMagic(header) {
		gzipReader, err := gzip.NewReader(peekReader)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		return struct {
			io.Reader
			io.Closer
		}{
			Reader: io.LimitReader(gzipReader, maxDecompressedSize),
			Closer: gzipReader,
		}, nil
	}

	return io.NopCloser(peekReader), nil
}

func writeTar(ctx context.Context, reader *tar.Reader, dst string) error {
	for {
		// If the context has been canceled or has timed out, return.
		if err := ctx.Err(); err != nil {
			return err
		}

		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		} else if err != nil {
			return err
		}

		slog.InfoContext(ctx, "Decompressing", "name", header.Name)

		path := filepath.Clean(header.Name)
		if !filepath.IsLocal(path) {
			return fmt.Errorf("refusing to write non-local path %q", path)
		}

		target := filepath.Join(dst, path)

		// Ensure that we can write the directory
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if header.Mode > math.MaxUint32 {
				return fmt.Errorf("invalid file mode for %q: %02x", header.Name, header.Mode)
			}

			fileMode := os.FileMode(header.Mode) //nolint:gosec // We checked the overflow
			err := os.Mkdir(target, fileMode)
			if err != nil && !errors.Is(err, fs.ErrExist) {
				return err
			}

		case tar.TypeReg:
			if header.Mode > math.MaxUint32 {
				return fmt.Errorf("invalid file mode for %q: %02x", header.Name, header.Mode)
			}

			fileMode := os.FileMode(header.Mode) //nolint:gosec // We checked the overflow
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, fileMode)
			if err != nil {
				return err
			}

			if err := func() (err error) {
				// We wrap this defer in an immediately invoked function
				// so that the file is closed within this loop iteration,
				// not at the end of writeTar.
				defer func() { err = errors.Join(err, f.Close()) }()
				// Write the tar file into f
				_, err = io.Copy(f, reader)
				return err
			}(); err != nil {
				return err
			}
		}
	}
}
