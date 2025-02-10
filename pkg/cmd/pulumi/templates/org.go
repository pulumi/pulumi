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

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func (s *Source) getOrgTemplates(ctx context.Context, interactive bool) error {
	ws := pkgWorkspace.Instance
	project, _, err := ws.ReadProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return err
	}

	b, err := cmdBackend.CurrentBackend(ctx, ws, cmdBackend.DefaultLoginManager, project, display.Options{
		Color:         cmdutil.GetGlobalColorization(),
		IsInteractive: interactive,
	})
	if err != nil {
		return err
	}

	logging.Infof("Listing Org Templates from the cloud")
	_, orgs, _, err := b.CurrentUser()
	if err != nil {
		return err
	}

	creds, err := workspace.GetStoredCredentials()
	if err != nil {
		return err
	}

	cloudURL, err := pkgWorkspace.GetCurrentCloudURL(ws, env.Global(), project)
	if err != nil {
		return fmt.Errorf("could not get cloud url: %w", err)
	}

	logging.Infof("Org Templates URL: %q", cloudURL)

	api := client.NewClient(cloudURL, creds.AccessTokens[creds.Current], false, cmdutil.Diag())

	alreadySeenSourceURLs := map[string]struct{}{}

	var errs []error
	for _, org := range orgs {
		logging.Infof("Checking for templates from %q", org)
		orgTemplates, err := api.ListOrgTemplates(ctx, org)
		if apiError := new(apitype.ErrorResponse); errors.As(err, &apiError) {
			// This is what happens when we try to access org templates for an org that hasn't enabled org templates.
			if apiError.Code == 402 {
				logging.Infof("%q does not have access to org templates (code=%d)", org, apiError.Code)
				continue
			}
		} else if err != nil {
			errs = append(errs, err)
			logging.Warningf("Failed to get templates from %q: %s", org, err.Error())
			continue
		} else if orgTemplates.HasAccessError {
			logging.Warningf("Failed to get templates from %q: Access Denied", org)
			continue
		} else if orgTemplates.HasUpstreamError {
			logging.Warningf("Failed to get templates from %q: Upstream Error", org)
			continue
		}

		for source, sourceTemplates := range orgTemplates.Templates {
			logging.Infof("sourcing templates from %q", source)
			for _, template := range sourceTemplates {
				// These template are maintained using https://github.com/pulumi/templates, and are
				// ingested without going through the Pulumi Cloud.
				//
				//
				if strings.HasPrefix(template.SourceURL, "https://github.com/pulumi/templates") {
					continue
				}

				// Check if we already have this template from another source.
				if _, ok := alreadySeenSourceURLs[template.SourceURL]; ok {
					// Skip a template that we have already seen.
					continue
				}
				alreadySeenSourceURLs[template.SourceURL] = struct{}{}
				logging.V(10).Infof("adding template %q", template.Name)
				s.templates = append(s.templates, Template{
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
						s.closers = append(s.closers, func() error { return os.RemoveAll(templateDir) })

						tarReader, err := api.DownloadOrgTemplate(ctx, org, template.SourceURL)
						if err != nil {
							return workspace.Template{}, err
						}
						if err := errors.Join(
							writeTar(&tarReader.Reader, templateDir),
							tarReader.Close(),
						); err != nil {
							return workspace.Template{}, err
						}

						return workspace.LoadTemplate(templateDir)
					},
				})
			}
		}
	}

	return errors.Join(errs...)
}

func writeTar(reader *tar.Reader, dst string) error {
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

		switch header.Typeflag {
		case tar.TypeDir:
			err := os.MkdirAll(target, 0o755)
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
