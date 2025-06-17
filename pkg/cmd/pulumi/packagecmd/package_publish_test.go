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

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/util/testutil"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // This test uses the global backendInstance variable
func TestPackagePublishCmd_Run(t *testing.T) {
	version := semver.MustParse("1.0.0")

	tests := []struct {
		name                string
		args                publishPackageArgs
		packageSource       string
		packageParams       []string
		mockSchema          *schema.Package
		schemaExtractionErr error
		mockOrg             string
		mockOrgErr          error
		publishErr          error
		expectedErr         string
		readmeContent       string
		installContent      string
		sourceDir           func(t *testing.T) string
		pluginDir           func(t *testing.T) string
	}{
		{
			name: "successful publish with publisher from schema",
			args: publishPackageArgs{
				source: "pulumi",
			},
			packageSource: "testpackage",
			packageParams: []string{},
			mockSchema: &schema.Package{
				Name:      "testpkg",
				Publisher: "testpublisher",
				Version:   &version,
				Provider:  &schema.Resource{},
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
			packageParams: []string{},
			mockSchema: &schema.Package{
				Name:     "testpkg",
				Version:  &version,
				Provider: &schema.Resource{},
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
			packageParams: []string{},
			mockSchema: &schema.Package{
				Name:     "testpkg",
				Version:  &version,
				Provider: &schema.Resource{},
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
			packageParams: []string{},
			mockSchema: &schema.Package{
				Name:     "testpkg",
				Version:  &version,
				Provider: &schema.Resource{},
			},
			readmeContent: "# Test README\nThis is a test readme.",
		},
		{
			name: "loads readme from package source",
			args: publishPackageArgs{
				source:    "pulumi",
				publisher: "publisher",
			},
			mockSchema: &schema.Package{
				Name:     "testpkg",
				Version:  &version,
				Provider: &schema.Resource{},
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
			packageParams: []string{},
			packageSource: "testpackage",
			mockSchema: &schema.Package{
				Name:     "testpackage",
				Version:  &version,
				Provider: &schema.Resource{},
			},
			pluginDir: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()
				testPlugin := path.Join(dir, "resource-testpackage")
				err := os.MkdirAll(testPlugin, 0o755)
				require.NoError(t, err)
				readmeFile, err := os.Create(path.Join(testPlugin, "README.md"))
				require.NoError(t, err)
				defer contract.IgnoreClose(readmeFile)
				_, err = readmeFile.WriteString("# README from the installed plugin\nThis is a test readme.")
				require.NoError(t, err)
				return dir
			},
		},
		{
			name: "error when no publisher available",
			args: publishPackageArgs{
				source: "pulumi",
			},
			packageSource: "testpackage",
			packageParams: []string{},
			mockSchema: &schema.Package{
				Name:     "testpkg",
				Version:  &version,
				Provider: &schema.Resource{},
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
			packageParams: []string{},
			mockSchema: &schema.Package{
				Name:     "testpkg",
				Version:  &version,
				Provider: &schema.Resource{},
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
			packageParams:  []string{},
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
			packageParams: []string{},
			mockSchema: &schema.Package{
				Version:  &version,
				Provider: &schema.Resource{},
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
			packageParams: []string{},
			mockSchema: &schema.Package{
				Name:     "testpkg",
				Provider: &schema.Resource{},
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
			packageParams: []string{},
			mockSchema: &schema.Package{
				Name:     "testpkg",
				Version:  &version,
				Provider: &schema.Resource{},
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
			mockSchema: &schema.Package{
				Name:     "testpkg",
				Version:  &version,
				Provider: &schema.Resource{},
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
			mockSchema: &schema.Package{
				Name:     "testpkg",
				Version:  &version,
				Provider: &schema.Resource{},
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
			packageParams: []string{},
			mockSchema: &schema.Package{
				Name:     "testpkg",
				Version:  &version,
				Provider: &schema.Resource{},
			},
			expectedErr: "failed to find readme: failed to create plugin spec: VERSION must be valid semver",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()

			packageSource := tt.packageSource
			var readmePath string
			var expectedReadmeContent string
			if tt.readmeContent != "" {
				readmeFile, err := os.Create(path.Join(tempDir, "readme.md"))
				require.NoError(t, err)
				_, err = readmeFile.WriteString(tt.readmeContent)
				require.NoError(t, err)
				require.NoError(t, readmeFile.Close())
				readmePath = readmeFile.Name()
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
			var pluginDir string
			if tt.pluginDir != nil {
				pluginDir = tt.pluginDir(t)
				readmePath = path.Join(pluginDir, "resource-"+tt.packageSource, "README.md")
				if readmeFile, err := os.Stat(readmePath); err == nil && !readmeFile.IsDir() {
					readmeData, err := os.ReadFile(readmePath)
					require.NoError(t, err)
					expectedReadmeContent = string(readmeData)
				}
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

			mockPackageRegistry := &backend.MockPackageRegistry{
				PublishF: func(ctx context.Context, op apitype.PackagePublishOp) error {
					schemaBytes, err := io.ReadAll(op.Schema)
					require.NoError(t, err)
					packageSpec, err := unmarshalSchema(schemaBytes)

					if len(packageSpec.Types) == 0 {
						packageSpec.Types = map[string]schema.ComplexTypeSpec{}
					}
					if len(packageSpec.Resources) == 0 {
						packageSpec.Resources = map[string]schema.ResourceSpec{}
					}
					if len(packageSpec.Functions) == 0 {
						packageSpec.Functions = map[string]schema.FunctionSpec{}
					}
					require.NoError(t, err)
					expectedSpec, err := tt.mockSchema.MarshalSpec()
					require.NoError(t, err)
					assert.Equal(t, expectedSpec, packageSpec, "package schema should match input package spec")

					// Verify readme and install docs content
					if tt.args.readmePath != "" {
						actualContents, err := io.ReadAll(op.Readme)
						require.NoError(t, err)
						assert.Equal(t, expectedReadmeContent, string(actualContents), "readme should match the provided markdown file")
					}
					if tt.args.installDocsPath != "" {
						actualContents, err := io.ReadAll(op.InstallDocs)
						require.NoError(t, err)
						assert.Equal(t, tt.installContent, string(actualContents), "install docs should match the provided markdown file")
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
				GetPackageRegistryF: func() (backend.PackageRegistry, error) {
					return mockPackageRegistry, nil
				},
			})

			// Setup defaultOrg mock
			defaultOrg := func(context.Context, backend.Backend, *workspace.Project) (string, error) {
				return tt.mockOrg, tt.mockOrgErr
			}

			cmd := &packagePublishCmd{
				defaultOrg: defaultOrg,
				extractSchema: func(pctx *plugin.Context, packageSource string, args []string) (*schema.Package, error) {
					if tt.mockSchema == nil && tt.schemaExtractionErr == nil {
						return nil, errors.New("mock schema extraction failed")
					}
					return tt.mockSchema, tt.schemaExtractionErr
				},
				pluginDir: pluginDir,
			}

			err := cmd.Run(context.Background(), tt.args, packageSource, tt.packageParams)
			if tt.expectedErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

//nolint:paralleltest // This test uses the global backendInstance variable
func TestPackagePublishCmd_IOErrors(t *testing.T) {
	t.Parallel()
	version := semver.MustParse("1.0.0")
	validSchema := &schema.Package{
		Name:      "testpkg",
		Publisher: "testpublisher",
		Version:   &version,
		Provider:  &schema.Resource{},
	}

	tests := []struct {
		name           string
		args           publishPackageArgs
		mockSchema     *schema.Package
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
				GetPackageRegistryF: func() (backend.PackageRegistry, error) {
					return &backend.MockPackageRegistry{
						PublishF: func(ctx context.Context, op apitype.PackagePublishOp) error {
							return nil
						},
					}, nil
				},
			})

			cmd := &packagePublishCmd{
				defaultOrg: func(context.Context, backend.Backend, *workspace.Project) (string, error) {
					return "default-org", nil
				},
				extractSchema: func(pctx *plugin.Context, packageSource string, args []string) (*schema.Package, error) {
					return tt.mockSchema, nil
				},
			}

			err := cmd.Run(context.Background(), tt.args, "testpackage", []string{})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErrStr)
		})
	}
}

//nolint:paralleltest // This test uses the global backendInstance variable
func TestPackagePublishCmd_BackendErrors(t *testing.T) {
	version := semver.MustParse("1.0.0")
	validSchema := &schema.Package{
		Name:      "testpkg",
		Publisher: "testpublisher",
		Version:   &version,
		Provider:  &schema.Resource{},
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
				extractSchema: func(pctx *plugin.Context, packageSource string, args []string) (*schema.Package, error) {
					return validSchema, nil
				},
			}

			err = cmd.Run(context.Background(), publishPackageArgs{
				source:     "pulumi",
				publisher:  "publisher",
				readmePath: readmePath,
			}, "testpackage", []string{})

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErrStr)
		})
	}
}

type mockWorkspace struct {
	readProjectErr error
}

var _ pkgWorkspace.Context = &mockWorkspace{}

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
		extractSchema: func(pctx *plugin.Context, packageSource string, args []string) (*schema.Package, error) {
			pkg := &schema.Package{
				Name:    "test-package",
				Version: &semver.Version{Major: 1, Minor: 0, Patch: 0},
			}
			return pkg, nil
		},
	}

	customErr := errors.New("custom project read error")
	originalWorkspace := pkgWorkspace.Instance
	t.Cleanup(func() { pkgWorkspace.Instance = originalWorkspace })
	pkgWorkspace.Instance = &mockWorkspace{readProjectErr: customErr}

	err := cmd.Run(context.Background(), publishPackageArgs{readmePath: "README.md"}, "test-source", []string{})

	assert.Error(t, err)
	assert.ErrorIs(t, err, customErr)
}

func unmarshalSchema(schemaBytes []byte) (*schema.PackageSpec, error) {
	var spec schema.PackageSpec

	err := json.Unmarshal(schemaBytes, &spec)
	return &spec, err
}

func TestFindReadme(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	ctx := context.Background()

	cmd := packagePublishCmd{
		pluginDir: tmpDir,
	}

	t.Run("NonExistentDirectory", func(t *testing.T) {
		t.Parallel()
		nonExistentDir := filepath.Join(tmpDir, "does-not-exist")
		readme, err := cmd.findReadme(ctx, nonExistentDir)
		assert.Empty(t, readme)
		assert.NoError(t, err, "Should not return error for non-existent directory")
	})

	t.Run("FileInsteadOfDirectory", func(t *testing.T) {
		t.Parallel()
		filePath := filepath.Join(tmpDir, "file.txt")
		err := os.WriteFile(filePath, []byte("not a readme"), 0o600)
		require.NoError(t, err)

		readme, err := cmd.findReadme(ctx, filePath)
		assert.Empty(t, readme)
		assert.NoError(t, err, "Should not return error when source is a file")
	})

	t.Run("SchemaFile", func(t *testing.T) {
		t.Parallel()
		schemaPath := filepath.Join(tmpDir, "schema.json")
		err := os.WriteFile(schemaPath, []byte("{}"), 0o600)
		require.NoError(t, err)

		readme, err := cmd.findReadme(ctx, schemaPath)
		assert.Empty(t, readme)
		assert.NoError(t, err, "Should not return error when source is a schema file")
	})

	t.Run("DirectoryWithoutReadme", func(t *testing.T) {
		t.Parallel()
		dirPath := filepath.Join(tmpDir, "no-readme-dir")
		require.NoError(t, os.Mkdir(dirPath, 0o755))

		readme, err := cmd.findReadme(ctx, dirPath)
		assert.Empty(t, readme)
		assert.NoError(t, err, "Should not return error when directory has no readme")
	})

	t.Run("DirectoryWithReadme", func(t *testing.T) {
		t.Parallel()
		dirPath := filepath.Join(tmpDir, "with-readme-dir")
		require.NoError(t, os.Mkdir(dirPath, 0o755))
		readmePath := filepath.Join(dirPath, "README.md")
		require.NoError(t, os.WriteFile(readmePath, []byte("# Test Readme"), 0o600))

		found, err := cmd.findReadme(ctx, dirPath)
		assert.Equal(t, readmePath, found)
		assert.NoError(t, err)
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
		assert.NoError(t, err, "Should not return error when no readme is found")
	})

	t.Run("Git Plugin Download URL", func(t *testing.T) {
		t.Parallel()
		pluginDownloadURL := "git://github.com/pulumi/pulumi-example@v1.2.3"
		pluginSpec, err := workspace.NewPluginSpec(ctx, pluginDownloadURL, apitype.ResourcePlugin, nil, "", nil)
		require.NoError(t, err)
		pluginSpec.PluginDir = cmd.pluginDir

		dirPath := filepath.Join(tmpDir, pluginSpec.Dir())
		require.NoError(t, os.Mkdir(dirPath, 0o755))
		readmePath := filepath.Join(dirPath, "README.md")
		require.NoError(t, os.WriteFile(readmePath, []byte("# Test Readme"), 0o600))

		readme, err := cmd.findReadme(ctx, pluginDownloadURL)
		assert.Equal(t, readmePath, readme)
		assert.NoError(t, err)
	})

	t.Run("Git Plugin Download URL with subdirectory", func(t *testing.T) {
		t.Parallel()
		pluginDownloadURL := "git://github.com/pulumi/pulumi-subdir-example/path@v1.2.3"
		pluginSpec, err := workspace.NewPluginSpec(ctx, pluginDownloadURL, apitype.ResourcePlugin, nil, "", nil)
		require.NoError(t, err)
		pluginSpec.PluginDir = cmd.pluginDir

		dirPath := filepath.Join(tmpDir, pluginSpec.Dir())
		require.NoError(t, os.Mkdir(dirPath, 0o755))
		readmePath := filepath.Join(dirPath, "README.md")
		require.NoError(t, os.WriteFile(readmePath, []byte("# Root Readme"), 0o600))
		subdirPath := filepath.Join(dirPath, "path")
		require.NoError(t, os.Mkdir(subdirPath, 0o755))
		subdirReadmePath := filepath.Join(subdirPath, "README.md")
		require.NoError(t, os.WriteFile(subdirReadmePath, []byte("# Subdir Readme"), 0o600))

		readme, err := cmd.findReadme(ctx, pluginDownloadURL)
		assert.Equal(t, subdirReadmePath, readme)
		assert.NoError(t, err)
	})
}
