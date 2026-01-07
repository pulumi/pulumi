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

package packagecmd

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/pluginstorage"
	"github.com/pulumi/pulumi/pkg/v3/util/testutil"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // This test uses the global backendInstance variable
func TestPackagePublishCmd_Run(t *testing.T) {
	tests := []struct {
		name                  string
		args                  publishPackageArgs
		packageSource         string
		packageParams         plugin.ParameterizeParameters
		mockSchema            *schema.PackageSpec
		schemaExtractionErr   error
		mockOrg               string
		mockOrgErr            error
		publishErr            error
		expectedErr           string
		readmeContent         string
		installContent        string
		sourceDir             func(t *testing.T) string
		installPlugin         func(t *testing.T)
		expectedReadmeContent string
	}{
		{
			name: "successful publish with publisher from schema",
			args: publishPackageArgs{
				source: "pulumi",
			},
			packageSource: "testpackage",
			mockSchema: &schema.PackageSpec{
				Name:      "testpkg",
				Publisher: "testpublisher",
				Version:   "1.0.0",
			},
			readmeContent:  "# Test README\nThis is a test readme.",
			installContent: "# Installation\nHow to install this package.",
		},
		{
			name: "successful publish with publisher from command line",
			args: publishPackageArgs{
				source:    "pulumi",
				publisher: "cmdpublisher",
			},
			packageSource: "testpackage",
			mockSchema: &schema.PackageSpec{
				Name:    "testpkg",
				Version: "1.0.0",
			},
			readmeContent:  "# Test README\nThis is a test readme.",
			installContent: "# Installation\nHow to install this package.",
		},
		{
			name: "successful publish with default org",
			args: publishPackageArgs{
				source: "pulumi",
			},
			packageSource: "testpackage",
			mockSchema: &schema.PackageSpec{
				Name:    "testpkg",
				Version: "1.0.0",
			},
			mockOrg:        "defaultorg",
			readmeContent:  "# Test README\nThis is a test readme.",
			installContent: "# Installation\nHow to install this package.",
		},
		{
			name: "successful publish without installation docs",
			args: publishPackageArgs{
				source:    "pulumi",
				publisher: "publisher",
			},
			packageSource: "testpackage",
			mockSchema: &schema.PackageSpec{
				Name:    "testpkg",
				Version: "1.0.0",
			},
			readmeContent: "# Test README\nThis is a test readme.",
		},
		{
			name: "loads readme from package source",
			args: publishPackageArgs{
				source:    "pulumi",
				publisher: "publisher",
			},
			mockSchema: &schema.PackageSpec{
				Name:    "testpkg",
				Version: "1.0.0",
			},
			sourceDir: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()
				readmeFile, err := os.Create(path.Join(dir, "README.md"))
				require.NoError(t, err)
				defer contract.IgnoreClose(readmeFile)
				_, err = readmeFile.WriteString("# README from the package source\nThis is a test readme.")
				require.NoError(t, err)
				return dir
			},
		},
		{
			name: "loads readme from installed plugin",
			args: publishPackageArgs{
				source:    "pulumi",
				publisher: "publisher",
			},
			packageSource: "testpackage",
			mockSchema: &schema.PackageSpec{
				Name:    "testpackage",
				Version: "1.0.0",
			},
			expectedReadmeContent: "# README from the installed plugin\nThis is a test readme.",
			installPlugin: func(t *testing.T) {
				t.Helper()

				pulumiHomeDir := t.TempDir() // Create an isolated PULUMI_HOME directory to install into
				t.Setenv(env.Home.Var().Name(), pulumiHomeDir)

				installResourcePluginFromFiles(t, workspace.PluginDescriptor{
					Name: "testpackage",
					Kind: apitype.ResourcePlugin,
				}, map[string]string{
					"README.md": "# README from the installed plugin\nThis is a test readme.",
				})
			},
		},
		{
			name: "error when no publisher available",
			args: publishPackageArgs{
				source: "pulumi",
			},
			packageSource: "testpackage",
			mockSchema: &schema.PackageSpec{
				Name:    "testpkg",
				Version: "1.0.0",
			},
			expectedErr:    "no publisher specified and no default organization found",
			readmeContent:  "# Test README\nThis is a test readme.",
			installContent: "# Installation\nHow to install this package.",
		},
		{
			name: "error when determining default org fails",
			args: publishPackageArgs{
				source: "pulumi",
			},
			packageSource: "testpackage",
			mockSchema: &schema.PackageSpec{
				Name:    "testpkg",
				Version: "1.0.0",
			},
			mockOrgErr:     errors.New("unexpected error"),
			expectedErr:    "failed to determine default organization: unexpected error",
			readmeContent:  "# Test README\nThis is a test readme.",
			installContent: "# Installation\nHow to install this package.",
		},
		{
			name: "error when extracting schema fails",
			args: publishPackageArgs{
				source:    "pulumi",
				publisher: "publisher",
			},
			packageSource:  "testpackage",
			mockSchema:     nil,
			expectedErr:    "failed to get schema",
			readmeContent:  "# Test README\nThis is a test readme.",
			installContent: "# Installation\nHow to install this package.",
		},
		{
			name: "error when no package name in schema",
			args: publishPackageArgs{
				source:    "pulumi",
				publisher: "publisher",
			},
			packageSource: "testpackage",
			mockSchema: &schema.PackageSpec{
				Version: "1.0.0",
			},
			expectedErr:    "no package name specified",
			readmeContent:  "# Test README\nThis is a test readme.",
			installContent: "# Installation\nHow to install this package.",
		},
		{
			name: "error when no version in schema",
			args: publishPackageArgs{
				source:    "pulumi",
				publisher: "publisher",
			},
			packageSource: "testpackage",
			mockSchema: &schema.PackageSpec{
				Name: "testpkg",
			},
			expectedErr:    "no version specified",
			readmeContent:  "# Test README\nThis is a test readme.",
			installContent: "# Installation\nHow to install this package.",
		},
		{
			name: "error when readme is omitted",
			args: publishPackageArgs{
				source:    "pulumi",
				publisher: "publisher",
			},
			packageSource: "testpackage",
			mockSchema: &schema.PackageSpec{
				Name:    "testpkg",
				Version: "1.0.0",
			},
			expectedErr: "no README found. Please add one named README.md to the package, or use --readme to specify the path",
		},
		{
			name: "error when publish fails",
			args: publishPackageArgs{
				source:    "pulumi",
				publisher: "publisher",
			},
			packageSource: "testpackage",
			mockSchema: &schema.PackageSpec{
				Name:    "testpkg",
				Version: "1.0.0",
			},
			publishErr:     errors.New("publish failed"),
			expectedErr:    "failed to publish package",
			readmeContent:  "# Test README\nThis is a test readme.",
			installContent: "# Installation\nHow to install this package.",
		},
		{
			name: "error when schema extraction fails",
			args: publishPackageArgs{
				source:    "pulumi",
				publisher: "publisher",
			},
			packageSource: "testpackage",
			mockSchema: &schema.PackageSpec{
				Name:    "testpkg",
				Version: "1.0.0",
			},
			schemaExtractionErr: errors.New("schema extraction failed"),
			expectedErr:         "failed to get schema: schema extraction failed",
			readmeContent:       "# Test README\nThis is a test readme.",
			installContent:      "# Installation\nHow to install this package.",
		},
		{
			name: "error when readme extraction fails",
			args: publishPackageArgs{
				source:    "pulumi",
				publisher: "publisher",
			},
			packageSource: "testpackage@not-a-valid-version",
			mockSchema: &schema.PackageSpec{
				Name:    "testpkg",
				Version: "1.0.0",
			},
			expectedErr: "failed to find readme: failed to create plugin spec: VERSION must be valid semver",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()

			packageSource := tt.packageSource
			var readmePath string
			expectedReadmeContent := tt.expectedReadmeContent
			if tt.readmeContent != "" {
				readmePath = filepath.Join(tempDir, "readme.md")
				err := os.WriteFile(filepath.Join(tempDir, "readme.md"), []byte(tt.readmeContent), 0o600)
				require.NoError(t, err)
				tt.args.readmePath = readmePath
				expectedReadmeContent = tt.readmeContent
			}
			if tt.sourceDir != nil {
				packageSource = tt.sourceDir(t)

				readmePath = path.Join(packageSource, "README.md")
				if readmeFile, err := os.Stat(readmePath); err == nil && !readmeFile.IsDir() {
					readmeData, err := os.ReadFile(readmePath)
					require.NoError(t, err)
					expectedReadmeContent = string(readmeData)
				}
			}
			if tt.installPlugin != nil {
				tt.installPlugin(t)
			}

			var installDocsPath string
			if tt.installContent != "" {
				installDocsFile, err := os.Create(path.Join(tempDir, "install.md"))
				require.NoError(t, err)
				_, err = installDocsFile.WriteString(tt.installContent)
				require.NoError(t, err)
				require.NoError(t, installDocsFile.Close())
				installDocsPath = installDocsFile.Name()
				tt.args.installDocsPath = installDocsPath
			}

			mockCloudRegistry := &backend.MockCloudRegistry{
				PublishPackageF: func(ctx context.Context, op apitype.PackagePublishOp) error {
					schemaBytes, err := io.ReadAll(op.Schema)
					require.NoError(t, err)
					packageSpec, err := unmarshalSchema(schemaBytes)

					require.NoError(t, err)
					assert.Equal(t, tt.mockSchema, packageSpec, "package schema should match input package spec")

					// Verify readme and install docs content
					actualReadme, err := io.ReadAll(op.Readme)
					require.NoError(t, err)
					assert.Equal(t, expectedReadmeContent, string(actualReadme), "readme should match the provided markdown file")

					if tt.args.installDocsPath != "" {
						actualInstallDocs, err := io.ReadAll(op.InstallDocs)
						require.NoError(t, err)
						assert.Equal(t, tt.installContent, string(actualInstallDocs),
							"install docs should match the provided markdown file")
					}

					// Verify publisher is set correctly
					if tt.args.publisher != "" {
						assert.Equal(t, tt.args.publisher, op.Publisher, "publisher should match command line argument")
					} else if tt.mockSchema.Publisher != "" {
						assert.Equal(t, tt.mockSchema.Publisher, op.Publisher, "publisher should match schema publisher")
					} else {
						assert.Equal(t, tt.mockOrg, op.Publisher, "publisher should match default org")
					}
					return tt.publishErr
				},
			}

			testutil.MockBackendInstance(t, &backend.MockBackend{
				GetCloudRegistryF: func() (backend.CloudRegistry, error) {
					return mockCloudRegistry, nil
				},
				GetReadOnlyCloudRegistryF: func() registry.Registry { return mockCloudRegistry },
			})

			// Setup defaultOrg mock
			defaultOrg := func(context.Context, backend.Backend, *workspace.Project) (string, error) {
				return tt.mockOrg, tt.mockOrgErr
			}

			cmd := &packagePublishCmd{
				defaultOrg: defaultOrg,
				extractSchema: func(
					pctx *plugin.Context, packageSource string, parameters plugin.ParameterizeParameters,
					registry registry.Registry, _ env.Env, _ int,
				) (*schema.PackageSpec, *workspace.PackageSpec, error) {
					if tt.mockSchema == nil && tt.schemaExtractionErr == nil {
						return nil, nil, errors.New("mock schema extraction failed")
					}
					return tt.mockSchema, nil, tt.schemaExtractionErr
				},
			}

			err := cmd.Run(t.Context(), tt.args, packageSource, tt.packageParams)
			if tt.expectedErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

//nolint:paralleltest // This test uses the global backendInstance variable
func TestPackagePublishCmd_IOErrors(t *testing.T) {
	t.Parallel()
	validSchema := &schema.PackageSpec{
		Name:      "testpkg",
		Publisher: "testpublisher",
		Version:   "1.0.0",
	}

	tests := []struct {
		name           string
		args           publishPackageArgs
		mockSchema     *schema.PackageSpec
		setupTest      func(*testing.T) (string, string)
		expectedErrStr string
	}{
		{
			name: "readme file not found",
			args: publishPackageArgs{
				source:     "pulumi",
				publisher:  "publisher",
				readmePath: "nonexistent-readme.md",
			},
			mockSchema:     validSchema,
			expectedErrStr: "failed to open readme file",
		},
		{
			name: "install docs file not found",
			args: publishPackageArgs{
				source:          "pulumi",
				publisher:       "publisher",
				installDocsPath: "nonexistent-install.md",
			},
			mockSchema: validSchema,
			setupTest: func(t *testing.T) (string, string) {
				tempDir := t.TempDir()
				readmePath := path.Join(tempDir, "readme.md")

				err := os.WriteFile(readmePath, []byte("# Test README"), 0o600)
				require.NoError(t, err)

				return readmePath, ""
			},
			expectedErrStr: "failed to open install docs file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupTest != nil {
				readmePath, installPath := tt.setupTest(t)
				if readmePath != "" {
					tt.args.readmePath = readmePath
				}
				if installPath != "" {
					tt.args.installDocsPath = installPath
				}
			}

			// Mock the backend
			testutil.MockBackendInstance(t, &backend.MockBackend{
				GetCloudRegistryF: func() (backend.CloudRegistry, error) {
					return &backend.MockCloudRegistry{
						PublishPackageF: func(ctx context.Context, op apitype.PackagePublishOp) error {
							return nil
						},
					}, nil
				},
				GetReadOnlyCloudRegistryF: func() registry.Registry { return &backend.MockCloudRegistry{} },
			})

			cmd := &packagePublishCmd{
				defaultOrg: func(context.Context, backend.Backend, *workspace.Project) (string, error) {
					return "default-org", nil
				},
				extractSchema: func(
					pctx *plugin.Context, packageSource string, parameters plugin.ParameterizeParameters,
					registry registry.Registry, _ env.Env, _ int,
				) (*schema.PackageSpec, *workspace.PackageSpec, error) {
					return tt.mockSchema, nil, nil
				},
			}

			err := cmd.Run(context.Background(), tt.args, "testpackage", nil /* packageParams */)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErrStr)
		})
	}
}

//nolint:paralleltest // This test uses the global backendInstance variable
func TestPackagePublishCmd_BackendErrors(t *testing.T) {
	validSchema := &schema.PackageSpec{
		Name:      "testpkg",
		Publisher: "testpublisher",
		Version:   "1.0.0",
	}

	tests := []struct {
		name           string
		setupBackend   func(t *testing.T)
		expectedErrStr string
	}{
		{
			name: "error getting package registry",
			setupBackend: func(t *testing.T) {
				testutil.MockBackendInstance(t, &backend.MockBackend{
					GetCloudRegistryF: func() (backend.CloudRegistry, error) {
						return nil, errors.New("failed to get package registry")
					},
					GetReadOnlyCloudRegistryF: func() registry.Registry {
						return &backend.MockCloudRegistry{}
					},
				})
			},
			expectedErrStr: "failed to get package registry",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary readme file
			tmpDir := t.TempDir()
			readmePath := path.Join(tmpDir, "readme.md")
			err := os.WriteFile(readmePath, []byte("# Test README"), 0o600)
			require.NoError(t, err)

			// Setup the test backend
			tt.setupBackend(t)

			cmd := &packagePublishCmd{
				defaultOrg: func(context.Context, backend.Backend, *workspace.Project) (string, error) {
					return "default-org", nil
				},
				extractSchema: func(
					pctx *plugin.Context, packageSource string, parameters plugin.ParameterizeParameters,
					registry registry.Registry, _ env.Env, _ int,
				) (*schema.PackageSpec, *workspace.PackageSpec, error) {
					return validSchema, nil, nil
				},
			}

			err = cmd.Run(context.Background(), publishPackageArgs{
				source:     "pulumi",
				publisher:  "publisher",
				readmePath: readmePath,
			}, "testpackage", nil /* packageParams */)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErrStr)
		})
	}
}

type mockWorkspace struct {
	readProjectErr error
}

var _ pkgWorkspace.Context = &mockWorkspace{}

func (m *mockWorkspace) New() (pkgWorkspace.W, error) {
	return nil, m.readProjectErr
}

func (m *mockWorkspace) ReadProject() (*workspace.Project, string, error) {
	return nil, "", m.readProjectErr
}

func (m *mockWorkspace) GetStoredCredentials() (workspace.Credentials, error) {
	return workspace.Credentials{}, nil
}

//nolint:paralleltest // This test uses the global pkgWorkspace.Instance variable
func TestPackagePublishCmd_Run_ReadProjectError(t *testing.T) {
	cmd := packagePublishCmd{
		defaultOrg: func(context.Context, backend.Backend, *workspace.Project) (string, error) {
			return "", nil
		},
		extractSchema: func(
			pctx *plugin.Context,
			packageSource string,
			parameters plugin.ParameterizeParameters,
			registry registry.Registry,
			_ env.Env, _ int,
		) (*schema.PackageSpec, *workspace.PackageSpec, error) {
			pkg := &schema.PackageSpec{
				Name:    "test-package",
				Version: "1.0.0",
			}
			return pkg, nil, nil
		},
	}

	customErr := errors.New("custom project read error")
	originalWorkspace := pkgWorkspace.Instance
	t.Cleanup(func() { pkgWorkspace.Instance = originalWorkspace })
	pkgWorkspace.Instance = &mockWorkspace{readProjectErr: customErr}

	err := cmd.Run(context.Background(), publishPackageArgs{readmePath: "README.md"},
		"test-source", nil /* packageParams */)

	assert.Error(t, err)
	assert.ErrorIs(t, err, customErr)
}

func unmarshalSchema(schemaBytes []byte) (*schema.PackageSpec, error) {
	var spec schema.PackageSpec

	err := json.Unmarshal(schemaBytes, &spec)
	return &spec, err
}

func TestFindReadme(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := t.Context()

	pulumiHomeDir := t.TempDir() // Create an isolated PULUMI_HOME directory to install into
	t.Setenv(env.Home.Var().Name(), pulumiHomeDir)

	cmd := packagePublishCmd{}

	t.Run("NonExistentDirectory", func(t *testing.T) {
		t.Parallel()
		nonExistentDir := filepath.Join(tmpDir, "does-not-exist")
		readme, err := cmd.findReadme(ctx, nonExistentDir)
		assert.Empty(t, readme)
		require.NoError(t, err, "Should not return error for non-existent directory")
	})

	t.Run("FileInsteadOfDirectory", func(t *testing.T) {
		t.Parallel()
		filePath := filepath.Join(tmpDir, "file.txt")
		err := os.WriteFile(filePath, []byte("not a readme"), 0o600)
		require.NoError(t, err)

		readme, err := cmd.findReadme(ctx, filePath)
		assert.Empty(t, readme)
		require.NoError(t, err, "Should not return error when source is a file")
	})

	t.Run("SchemaFile", func(t *testing.T) {
		t.Parallel()
		schemaPath := filepath.Join(tmpDir, "schema.json")
		err := os.WriteFile(schemaPath, []byte("{}"), 0o600)
		require.NoError(t, err)

		readme, err := cmd.findReadme(ctx, schemaPath)
		assert.Empty(t, readme)
		require.NoError(t, err, "Should not return error when source is a schema file")
	})

	t.Run("DirectoryWithoutReadme", func(t *testing.T) {
		t.Parallel()
		dirPath := filepath.Join(tmpDir, "no-readme-dir")
		require.NoError(t, os.Mkdir(dirPath, 0o755))

		readme, err := cmd.findReadme(ctx, dirPath)
		assert.Empty(t, readme)
		require.NoError(t, err, "Should not return error when directory has no readme")
	})

	t.Run("DirectoryWithReadme", func(t *testing.T) {
		t.Parallel()
		dirPath := filepath.Join(tmpDir, "with-readme-dir")
		require.NoError(t, os.Mkdir(dirPath, 0o755))
		readmePath := filepath.Join(dirPath, "README.md")
		require.NoError(t, os.WriteFile(readmePath, []byte("# Test Readme"), 0o600))

		found, err := cmd.findReadme(ctx, dirPath)
		assert.Equal(t, readmePath, found)
		require.NoError(t, err)
	})

	t.Run("InvalidPluginSpec", func(t *testing.T) {
		t.Parallel()
		// An invalid plugin spec string should return an error
		invalidPlugin := "my-cool-plugin@not-a-valid-version"
		readme, err := cmd.findReadme(ctx, invalidPlugin)
		assert.Empty(t, readme)
		assert.Error(t, err, "Should return error for invalid plugin spec")
		assert.Contains(t, err.Error(), "failed to create plugin spec")
	})

	t.Run("NoReadmeFound", func(t *testing.T) {
		t.Parallel()
		// Use a valid-looking plugin name but with no readme
		validPlugin := "my-cool-plugin"
		readme, err := cmd.findReadme(ctx, validPlugin)
		assert.Empty(t, readme)
		require.NoError(t, err, "Should not return error when no readme is found")
	})

	t.Run("Git Plugin Download URL", func(t *testing.T) {
		t.Parallel()
		pluginDownloadURL := "git://github.com/pulumi/pulumi-example@v1.2.3"
		pluginSpec, err := workspace.NewPluginDescriptor(ctx, pluginDownloadURL, apitype.ResourcePlugin, nil, "", nil)
		require.NoError(t, err)

		installResourcePluginFromFiles(t, pluginSpec, map[string]string{
			"README.md": "# Test Readme",
		})

		readme, err := cmd.findReadme(ctx, pluginDownloadURL)
		require.NoError(t, err)
		actualReadme, err := os.ReadFile(readme)
		require.NoError(t, err)
		assert.Equal(t, "# Test Readme", string(actualReadme))
	})

	t.Run("Git Plugin Download URL with subdirectory", func(t *testing.T) {
		t.Parallel()
		pluginDownloadURL := "git://github.com/pulumi/pulumi-subdir-example/path@v1.2.3"
		pluginSpec, err := workspace.NewPluginDescriptor(ctx, pluginDownloadURL, apitype.ResourcePlugin, nil, "", nil)
		require.NoError(t, err)

		installResourcePluginFromFiles(t, pluginSpec, map[string]string{
			"README.md":      "# Root Readme",
			"path/README.md": "# Subdir Readme",
		})

		readme, err := cmd.findReadme(ctx, pluginDownloadURL)
		require.NoError(t, err)
		actualReadme, err := os.ReadFile(readme)
		require.NoError(t, err)
		assert.Equal(t, "# Subdir Readme", string(actualReadme))
	})
}

// installResourcePluginFromFiles installs into the **global** PULUMI_HOME the files
// described as spec.
//
// This function should only be used after t.Setenv(workspace.PulumiHomeEnvVar,
// pulumiHomeDir) has been called.
func installResourcePluginFromFiles(t *testing.T, spec workspace.PluginDescriptor, files map[string]string) {
	t.Helper()
	dir := t.TempDir()
	for path, content := range files {
		path = filepath.Join(dir, path)
		err := os.MkdirAll(filepath.Dir(path), 0o755)
		require.NoError(t, err)
		err = os.WriteFile(path, []byte(content), 0o600)
		require.NoError(t, err)
	}
	err := pkgWorkspace.InstallPluginContent(t.Context(), spec, pluginstorage.DirPlugin(dir), true)
	require.NoError(t, err)
}
