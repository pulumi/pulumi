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
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func (s *Source) getCloudTemplates(
	ctx context.Context, templateName string,
	wg *sync.WaitGroup, e env.Env,
) {
	if !e.GetBool(env.DisableRegistryResolve) && e.GetBool(env.Experimental) {
		s.getRegistryTemplates(ctx, e, templateName)
		return
	}

	// Use the old org templates based API.
	//
	// This path can be removed when we are confident in registry resolution. We will
	// always need to maintain a way to access templates without the service, but we
	// should only need to maintain one way to access templates through the service.
	s.getOrgTemplates(ctx, templateName, wg, e)
}

func (s *Source) getRegistryTemplates(ctx context.Context, e env.Env, templateName string) {
	r := cmdCmd.NewDefaultRegistry(ctx, pkgWorkspace.Instance, nil, cmdutil.Diag(), e)

	// Since the templates names displayed here differ from the template names
	// returned from ListTemplates for VCS backed templates, we need to fetch
	// all templates and then filter manually.
	var nameFilter *string

	for template, err := range r.ListTemplates(ctx, nameFilter) {
		if err != nil {
			s.addError(fmt.Errorf("could not get template: %w", err))
			return
		}

		if template.Source == "github" && strings.HasPrefix(template.Name, "pulumi/templates/") {
			// These template are maintained using https://github.com/pulumi/templates, and are
			// ingested without going through the Pulumi Cloud.
			continue
		}

		t := registryTemplate{template, r, s}
		if templateName != "" && t.Name() != templateName {
			continue
		}

		s.addTemplate(t)
	}
}

type registryTemplate struct {
	t        apitype.TemplateMetadata
	registry registry.Registry
	source   *Source
}

func (r registryTemplate) Name() string {
	switch r.t.Source {
	case "github", "gitlab":
		parts := strings.SplitN(r.t.Name, "/", 3)
		return parts[len(parts)-1]
	default:
		return r.t.Name
	}
}

func (r registryTemplate) Description() string {
	return ""
}

func (r registryTemplate) ProjectDescription() string {
	if r.t.Description == nil {
		return ""
	}
	return *r.t.Description
}

func (r registryTemplate) Error() error { return nil }

func (r registryTemplate) Download(ctx context.Context) (workspace.Template, error) {
	templateBytes, err := r.registry.DownloadTemplate(ctx, r.t.DownloadURL)
	if err != nil {
		return workspace.Template{}, fmt.Errorf("failed to download from %q: %w", r.t.DownloadURL, err)
	}
	defer contract.IgnoreClose(templateBytes)
	templateDir, err := os.MkdirTemp("", "pulumi-template-")
	if err != nil {
		return workspace.Template{}, fmt.Errorf("failed to make temporary directory: %w", err)
	}
	// Having created a template directory, we now add it to the list of directories to close.
	r.source.addCloser(func() error { return os.RemoveAll(templateDir) })
	if err := writeTar(ctx, tar.NewReader(templateBytes), templateDir); err != nil {
		return workspace.Template{}, err
	}

	template, err := workspace.LoadTemplate(templateDir)
	return template, err
}

func (s *Source) getOrgTemplates(
	ctx context.Context, templateName string,
	wg *sync.WaitGroup, e env.Env,
) {
	ws := pkgWorkspace.Instance
	project, _, err := ws.ReadProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		s.addError(fmt.Errorf("could not read the current project: %w", err))
		return
	}

	url, err := pkgWorkspace.GetCurrentCloudURL(ws, e, project)
	if err != nil {
		s.addError(fmt.Errorf("could not get current cloud url: %w", err))
		return
	}

	b, err := cmdBackend.DefaultLoginManager.Current(ctx, ws, cmdutil.Diag(), url, project, false)
	if err != nil {
		if !errors.Is(err, backenderr.MissingEnvVarForNonInteractiveError{}) {
			s.addError(fmt.Errorf("could not get the current backend: %w", err))
		}
		logging.Infof("could not get a backend for org templates")
		return
	} else if b == nil {
		logging.Infof("no current logged in user")
		return
	}

	// Attempt to retrieve the current user
	if _, _, _, err := b.CurrentUser(); err != nil {
		if errors.Is(err, backenderr.ErrLoginRequired) {
			logging.Infof("user is not logged in")
			return // No current user - so don't proceed
		}
		s.addError(fmt.Errorf("could not get the current user for %s: %s", url, err))
		return
	}

	if !b.SupportsTemplates() {
		logging.Infof("%s does not support Org Templates", b.Name())
		return
	}

	logging.Infof("Listing Org Templates from the cloud")
	user, orgs, _, err := b.CurrentUser()
	if err != nil {
		s.addError(fmt.Errorf("could not get the current user: %w", err))
		return
	} else if user == "" {
		return // No current user - so don't proceed.
	}

	alreadySeenSourceURLs := map[string]struct{}{}

	handleOrg := func(org string) {
		defer wg.Done()
		logging.Infof("Checking for templates from %q", org)
		orgTemplates, err := b.ListTemplates(ctx, org)
		if apiError := new(apitype.ErrorResponse); errors.As(err, &apiError) {
			// This is what happens when we try to access org templates for an org that hasn't enabled org templates.
			if apiError.Code == 402 {
				logging.Infof("%q does not have access to org templates (code=%d)", org, apiError.Code)
				return
			}
		} else if err != nil {
			s.addError(fmt.Errorf("list templates: %w", err))
			logging.Warningf("Failed to get templates from %q: %s", org, err.Error())
			return
		} else if orgTemplates.HasAccessError {
			logging.Warningf("Failed to get templates from %q: Access Denied\n"+
				"Please check that %s can access all template sources", org, b.Name())
			return
		} else if orgTemplates.HasUpstreamError {
			// This is a catch-all error indicating only that *something* went
			// wrong with fetching templates for an org.
			logging.Warningf("Failed to get templates from %q: %s could not download the template", org, b.Name())
			return
		}

		for source, sourceTemplates := range orgTemplates.Templates {
			logging.Infof("sourcing templates from %q", source)
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
					logging.V(10).Infof("skipping template %q", template.Name)
					continue
				}

				logging.V(10).Infof("adding template %q", template.Name)
				s.addTemplate(orgTemplate{
					t:       template,
					org:     org,
					source:  s,
					backend: b,
				})
			}
		}
	}

	for _, org := range orgs {
		wg.Add(1)
		go handleOrg(org)
	}
}

type orgTemplate struct {
	t       *apitype.PulumiTemplateRemote
	org     string
	source  *Source
	backend backend.Backend
}

func (t orgTemplate) Name() string               { return t.t.Name }
func (t orgTemplate) Description() string        { return "" }
func (t orgTemplate) ProjectDescription() string { return t.t.Description }
func (t orgTemplate) Error() error               { return nil }
func (t orgTemplate) Download(ctx context.Context) (workspace.Template, error) {
	templateDir, err := os.MkdirTemp("", "pulumi-template-")
	if err != nil {
		return workspace.Template{}, err
	}
	// Having created a template directory, we now add it to the list of directories to close.
	t.source.addCloser(func() error { return os.RemoveAll(templateDir) })

	tarReader, err := t.backend.DownloadTemplate(ctx, t.org, t.t.TemplateURL)
	if err != nil {
		return workspace.Template{}, err
	}
	if err := errors.Join(
		writeTar(ctx, tarReader.Tar(), templateDir),
		tarReader.Close(),
	); err != nil {
		return workspace.Template{}, err
	}
	logging.Infof("downloaded %q into %q", t.t.Name, templateDir)

	return workspace.LoadTemplate(templateDir)
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

		logging.V(8).Infof("Decompressing %q", header.Name)

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
