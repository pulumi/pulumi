// Copyright 2025, Pulumi Corporation.
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

package templatecmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"path"
	"testing"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/util/testutil"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // This test uses the global backend variable
func TestTemplatePublishCmd_Run(t *testing.T) {
	tests := []struct {
		name            string
		args            publishTemplateArgs
		templateDir     func(t *testing.T) string
		mockOrg         string
		mockOrgErr      error
		publishErr      error
		expectedErr     string
		validateArchive func(t *testing.T, archive io.Reader)
	}{
		{
			name: "successful publish with publisher from command line",
			args: publishTemplateArgs{
				publisher: "testpublisher",
				name:      "test-template",
				version:   "1.0.0",
			},
			templateDir: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()

				err := os.WriteFile(path.Join(dir, "Pulumi.yaml"), []byte(`name: test-template
runtime: nodejs
`), 0o600)
				require.NoError(t, err)

				err = os.WriteFile(path.Join(dir, "index.ts"), []byte(`import * as pulumi from "@pulumi/pulumi";

export const message = "Hello, Pulumi!";
`), 0o600)
				require.NoError(t, err)

				return dir
			},
			validateArchive: func(t *testing.T, archive io.Reader) {
				content, err := io.ReadAll(archive)
				require.NoError(t, err)
				assert.Greater(t, len(content), 0, "archive should not be empty")
			},
		},
		{
			name: "successful publish with default org",
			args: publishTemplateArgs{
				name:    "test-template",
				version: "1.0.0",
			},
			templateDir: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()

				err := os.WriteFile(path.Join(dir, "Pulumi.yaml"), []byte(`name: test-template
runtime: nodejs
`), 0o600)
				require.NoError(t, err)

				return dir
			},
			mockOrg: "defaultorg",
			validateArchive: func(t *testing.T, archive io.Reader) {
				content, err := io.ReadAll(archive)
				require.NoError(t, err)
				assert.Greater(t, len(content), 0, "archive should not be empty")
			},
		},
		{
			name: "error when no publisher available",
			args: publishTemplateArgs{
				name:    "test-template",
				version: "1.0.0",
			},
			templateDir: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()

				err := os.WriteFile(path.Join(dir, "Pulumi.yaml"), []byte(`name: test-template
runtime: nodejs
`), 0o600)
				require.NoError(t, err)

				return dir
			},
			expectedErr: "no publisher specified and no default organization found",
		},
		{
			name: "error when determining default org fails",
			args: publishTemplateArgs{
				name:    "test-template",
				version: "1.0.0",
			},
			templateDir: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()

				err := os.WriteFile(path.Join(dir, "Pulumi.yaml"), []byte(`name: test-template
runtime: nodejs
`), 0o600)
				require.NoError(t, err)

				return dir
			},
			mockOrgErr:  errors.New("unexpected error"),
			expectedErr: "failed to determine default organization: unexpected error",
		},
		{
			name: "error when template directory does not exist",
			args: publishTemplateArgs{
				publisher: "testpublisher",
				name:      "test-template",
				version:   "1.0.0",
			},
			templateDir: func(t *testing.T) string {
				return "/path/that/does/not/exist"
			},
			expectedErr: "template directory does not exist",
		},
		{
			name: "error when version is invalid",
			args: publishTemplateArgs{
				publisher: "testpublisher",
				name:      "test-template",
				version:   "not-a-version",
			},
			templateDir: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()

				err := os.WriteFile(path.Join(dir, "Pulumi.yaml"), []byte(`name: test-template
runtime: nodejs
`), 0o600)
				require.NoError(t, err)

				return dir
			},
			expectedErr: "invalid version format",
		},
		{
			name: "error when template name is invalid",
			args: publishTemplateArgs{
				publisher: "testpublisher",
				name:      "INVALID-NAME!!!",
				version:   "1.0.0",
			},
			templateDir: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()

				err := os.WriteFile(path.Join(dir, "Pulumi.yaml"), []byte(`name: test-template
runtime: nodejs
`), 0o600)
				require.NoError(t, err)

				return dir
			},
			expectedErr: "invalid template name",
		},
		{
			name: "error when publish fails",
			args: publishTemplateArgs{
				publisher: "testpublisher",
				name:      "test-template",
				version:   "1.0.0",
			},
			templateDir: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()

				err := os.WriteFile(path.Join(dir, "Pulumi.yaml"), []byte(`name: test-template
runtime: nodejs
`), 0o600)
				require.NoError(t, err)

				return dir
			},
			publishErr:  errors.New("publish failed"),
			expectedErr: "failed to publish template",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var templateDir string
			if tt.templateDir != nil {
				templateDir = tt.templateDir(t)
			}

			mockCloudRegistry := &backend.MockCloudRegistry{
				PublishTemplateF: func(ctx context.Context, op apitype.TemplatePublishOp) error {
					assert.Equal(t, "private", op.Source, "source should be 'private'")

					if tt.args.publisher != "" {
						assert.Equal(t, tt.args.publisher, op.Publisher, "publisher should match command line argument")
					} else {
						assert.Equal(t, tt.mockOrg, op.Publisher, "publisher should match default org")
					}

					assert.Equal(t, tt.args.name, op.Name, "name should match command line argument")

					expectedVersion, err := semver.ParseTolerant(tt.args.version)
					if err == nil {
						assert.Equal(t, expectedVersion, op.Version, "version should match command line argument")
					}

					if tt.validateArchive != nil {
						tt.validateArchive(t, op.Archive)
					}

					return tt.publishErr
				},
			}

			testutil.MockBackendInstance(t, &backend.MockBackend{
				GetCloudRegistryF: func() (backend.CloudRegistry, error) {
					return mockCloudRegistry, nil
				},
			})

			defaultOrg := func(context.Context, backend.Backend, *workspace.Project) (string, error) {
				return tt.mockOrg, tt.mockOrgErr
			}

			cmd := &templatePublishCmd{
				defaultOrg: defaultOrg,
			}

			mockCmd := &cobra.Command{}
			err := cmd.Run(context.Background(), mockCmd, tt.args, templateDir)
			if tt.expectedErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

//nolint:paralleltest // This test uses the global backend variable
func TestTemplatePublishCmd_BackendErrors(t *testing.T) {
	tests := []struct {
		name           string
		setupBackend   func(t *testing.T)
		expectedErrStr string
	}{
		{
			name: "error getting cloud registry",
			setupBackend: func(t *testing.T) {
				testutil.MockBackendInstance(t, &backend.MockBackend{
					GetCloudRegistryF: func() (backend.CloudRegistry, error) {
						return nil, errors.New("failed to get cloud registry")
					},
				})
			},
			expectedErrStr: "failed to get cloud registry",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			err := os.WriteFile(path.Join(tmpDir, "Pulumi.yaml"), []byte(`name: test-template
runtime: nodejs
`), 0o600)
			require.NoError(t, err)

			tt.setupBackend(t)

			cmd := &templatePublishCmd{
				defaultOrg: func(context.Context, backend.Backend, *workspace.Project) (string, error) {
					return "default-org", nil
				},
			}

			mockCmd := &cobra.Command{}
			err = cmd.Run(context.Background(), mockCmd, publishTemplateArgs{
				publisher: "publisher",
				name:      "test-template",
				version:   "1.0.0",
			}, tmpDir)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErrStr)
		})
	}
}

type mockTemplateWorkspace struct {
	readProjectErr error
}

var _ pkgWorkspace.Context = &mockTemplateWorkspace{}

func (m *mockTemplateWorkspace) ReadProject() (*workspace.Project, string, error) {
	return nil, "", m.readProjectErr
}

func (m *mockTemplateWorkspace) GetStoredCredentials() (workspace.Credentials, error) {
	return workspace.Credentials{}, nil
}

//nolint:paralleltest // This test uses the global pkgWorkspace.Instance variable
func TestTemplatePublishCmd_Run_ReadProjectError(t *testing.T) {
	cmd := templatePublishCmd{
		defaultOrg: func(context.Context, backend.Backend, *workspace.Project) (string, error) {
			return "", nil
		},
	}

	customErr := errors.New("custom project read error")
	originalWorkspace := pkgWorkspace.Instance
	t.Cleanup(func() { pkgWorkspace.Instance = originalWorkspace })
	pkgWorkspace.Instance = &mockTemplateWorkspace{readProjectErr: customErr}

	tmpDir := t.TempDir()
	err := os.WriteFile(path.Join(tmpDir, "Pulumi.yaml"), []byte(`name: test-template
runtime: nodejs
`), 0o600)
	require.NoError(t, err)

	mockCmd := &cobra.Command{}
	err = cmd.Run(context.Background(), mockCmd, publishTemplateArgs{
		publisher: "publisher",
		name:      "test-template",
		version:   "1.0.0",
	}, tmpDir)

	assert.Error(t, err)
	assert.ErrorIs(t, err, customErr)
}

//nolint:paralleltest // This test uses the global backend variable
func TestTemplatePublishCmd_ArchiveCreation(t *testing.T) {
	tests := []struct {
		name          string
		setupDir      func(t *testing.T) string
		expectedFiles []string
		excludedFiles []string
	}{
		{
			name: "archive creation with gitignore exclusions",
			setupDir: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()

				err := os.WriteFile(path.Join(dir, ".gitignore"), []byte(`node_modules/
*.log
.env
dist/
`), 0o600)
				require.NoError(t, err)

				require.NoError(t, os.WriteFile(path.Join(dir, "Pulumi.yaml"), []byte(`
          name: test-template
          runtime: python
        `), 0o600))
				require.NoError(t, os.WriteFile(path.Join(dir, "README.md"), []byte("# Test Template"), 0o600))
				require.NoError(t, os.WriteFile(path.Join(dir, "index.ts"), []byte(`export const message = "Hello";`), 0o600))
				require.NoError(t, os.Mkdir(path.Join(dir, "node_modules"), 0o755))
				require.NoError(t, os.WriteFile(path.Join(dir, "node_modules", "package.json"), []byte("{}"), 0o600))
				require.NoError(t, os.Mkdir(path.Join(dir, "dist"), 0o755))
				require.NoError(t, os.WriteFile(path.Join(dir, "dist", "index.js"), []byte("console.log('built');"), 0o600))
				require.NoError(t, os.WriteFile(path.Join(dir, "debug.log"), []byte("debug info"), 0o600))
				require.NoError(t, os.WriteFile(path.Join(dir, ".env"), []byte("SECRET=value"), 0o600))

				return dir
			},
			expectedFiles: []string{
				".gitignore",
				"Pulumi.yaml",
				"README.md",
				"index.ts",
			},
			excludedFiles: []string{
				"node_modules/package.json",
				"dist/index.js",
				"debug.log",
				".env",
			},
		},
		{
			name: "archive creation without gitignore",
			setupDir: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()

				require.NoError(t, os.WriteFile(path.Join(dir, "Pulumi.yaml"), []byte(`
          name: test-template
          runtime: python
        `), 0o600))
				require.NoError(t, os.WriteFile(path.Join(dir, "__main__.py"), []byte(`import pulumi`), 0o600))
				require.NoError(t, os.WriteFile(path.Join(dir, "requirements.txt"), []byte("pulumi>=3.0.0"), 0o600))
				return dir
			},
			expectedFiles: []string{
				"Pulumi.yaml",
				"__main__.py",
				"requirements.txt",
			},
			excludedFiles: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			templateDir := tt.setupDir(t)

			mockCloudRegistry := &backend.MockCloudRegistry{
				PublishTemplateF: func(ctx context.Context, op apitype.TemplatePublishOp) error {
					archiveBytes, err := io.ReadAll(op.Archive)
					require.NoError(t, err)
					assert.Greater(t, len(archiveBytes), 0, "archive should not be empty")

					reader := bytes.NewReader(archiveBytes)
					gzr, err := gzip.NewReader(reader)
					require.NoError(t, err)
					defer gzr.Close()

					tarReader := tar.NewReader(gzr)
					var actualEntries []string

					for {
						header, err := tarReader.Next()
						if err == io.EOF {
							break
						}
						require.NoError(t, err)
						actualEntries = append(actualEntries, header.Name)
					}

					for _, expectedFile := range tt.expectedFiles {
						assert.Contains(t, actualEntries, expectedFile, "expected file %s should be in archive", expectedFile)
					}

					for _, excludedFile := range tt.excludedFiles {
						assert.NotContains(t, actualEntries, excludedFile, "excluded file %s should not be in archive", excludedFile)
					}

					return nil
				},
			}

			testutil.MockBackendInstance(t, &backend.MockBackend{
				GetCloudRegistryF: func() (backend.CloudRegistry, error) {
					return mockCloudRegistry, nil
				},
			})

			cmd := &templatePublishCmd{
				defaultOrg: func(context.Context, backend.Backend, *workspace.Project) (string, error) {
					return "default-org", nil
				},
			}

			mockCmd := &cobra.Command{}
			err := cmd.Run(context.Background(), mockCmd, publishTemplateArgs{
				publisher: "testpublisher",
				name:      "test-template",
				version:   "1.0.0",
			}, templateDir)

			assert.NoError(t, err)
		})
	}
}
