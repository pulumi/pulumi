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
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func (s *Source) getOrgTemplates(
	ctx context.Context, templateName string,
	interactive bool, wg *sync.WaitGroup,
) {
	ws := pkgWorkspace.Instance
	project, _, err := ws.ReadProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		s.addError(err)
		return
	}

	b, err := cmdBackend.CurrentBackend(ctx, ws, cmdBackend.DefaultLoginManager, project, display.Options{
		Color:         cmdutil.GetGlobalColorization(),
		IsInteractive: interactive,
	})
	if err != nil {
		if !errors.Is(err, backend.MissingEnvVarForNonInteractiveError{}) {
			s.addError(err)
		}
		return
	}
	if !b.SupportsTemplates() {
		return
	}

	logging.Infof("Listing Org Templates from the cloud")
	user, orgs, _, err := b.CurrentUser()
	if err != nil {
		s.addError(err)
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
			s.addError(err)
			logging.Warningf("Failed to get templates from %q: %s", org, err.Error())
			return
		} else if orgTemplates.HasAccessError {
			logging.Warningf("Failed to get templates from %q: Access Denied", org)
			return
		} else if orgTemplates.HasUpstreamError {
			logging.Warningf("Failed to get templates from %q: Upstream Error", org)
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
				s.addTemplate(Template{
					name:               template.Name,
					description:        "", // No description present.
					projectDescription: template.Description,
					source:             s,
					download: func(ctx context.Context) (workspace.Template, error) {
						templateDir, err := os.MkdirTemp("", "pulumi-template-")
						if err != nil {
							return workspace.Template{}, err
						}
						// Having created a template directory, we now add it to the list of directories to close.
						s.addCloser(func() error { return os.RemoveAll(templateDir) })

						tarReader, err := b.DownloadTemplate(ctx, org, template.TemplateURL)
						if err != nil {
							return workspace.Template{}, err
						}
						if err := errors.Join(
							writeTar(ctx, tarReader.Tar(), templateDir),
							tarReader.Close(),
						); err != nil {
							return workspace.Template{}, err
						}
						logging.Infof("downloaded %q into %q", template.Name, templateDir)

						return workspace.LoadTemplate(templateDir)
					},
				})
			}
		}
	}

	for _, org := range orgs {
		wg.Add(1)
		go handleOrg(org)
	}
}

func writeTar(_ context.Context, reader *tar.Reader, dst string) error {
	for {
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

			// Write the tar file into f
			if _, err := io.Copy(f, reader); err != nil {
				return errors.Join(err, f.Close())
			}

			// Close f. We don't use defer because we want to close each file
			// after we open it, not leave it all until the download function
			// exits.
			if err := f.Close(); err != nil {
				return err
			}
		}
	}
}
