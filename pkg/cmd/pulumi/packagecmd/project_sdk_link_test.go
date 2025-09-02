// Copyright 2024, Pulumi Corporation.
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
	"bytes"
	"context"
	"iter"
	"os"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/codegen/nodejs"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrintNodeJsImportInstructions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		pkg            *schema.Package
		options        map[string]interface{}
		wantImportLine string
	}{
		{
			name: "uses package info name when available",
			pkg: &schema.Package{
				Name: "aws-native",
				Language: map[string]interface{}{
					"nodejs": nodejs.NodePackageInfo{
						PackageName: "@pulumi/aws-native-renamed",
					},
				},
			},
			options:        map[string]interface{}{},
			wantImportLine: "import * as awsNative from \"@pulumi/aws-native-renamed\";\n",
		},
		{
			name: "falls back to camelCase when no package info",
			pkg: &schema.Package{
				Name: "aws-native",
			},
			options:        map[string]interface{}{},
			wantImportLine: "import * as awsNative from \"@pulumi/aws-native\";\n",
		},
		{
			name: "respects typescript option",
			pkg: &schema.Package{
				Name: "aws-native",
			},
			options: map[string]interface{}{
				"typescript": false,
			},
			wantImportLine: "  const awsNative = require(\"@pulumi/aws-native\");\n",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			err := printNodeJsImportInstructions(&buf, tt.pkg, tt.options)
			require.NoError(t, err)

			output := buf.String()
			assert.Contains(t, output, tt.wantImportLine, "output should contain the import line")
		})
	}
}

func TestSetSpecNamespace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		pluginDownloadURL string
		wantNamespace     string
	}{
		{
			pluginDownloadURL: "https://pulumi.com/terraform/v1.0.0",
			wantNamespace:     "",
		},
		{
			pluginDownloadURL: "git://github.com/pulumi/pulumi-terraform",
			wantNamespace:     "pulumi",
		},
		{
			pluginDownloadURL: "git://",
			wantNamespace:     "",
		},
		{
			pluginDownloadURL: "git://example.com",
			wantNamespace:     "",
		},
		{
			pluginDownloadURL: "git://example.com/pulumi",
			wantNamespace:     "",
		},
		{
			pluginDownloadURL: "git://example.com/pulumi/a/long/path",
			wantNamespace:     "pulumi",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.pluginDownloadURL, func(t *testing.T) {
			t.Parallel()

			pluginSpec := workspace.PluginSpec{
				PluginDownloadURL: tt.pluginDownloadURL,
			}
			schemaSpec := &schema.PackageSpec{}
			setSpecNamespace(schemaSpec, pluginSpec)
			assert.Equal(t, tt.wantNamespace, schemaSpec.Namespace)
		})
	}
}

func TestTryRegistryResolution(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	pulumiYaml := `name: test-project
runtime: nodejs
packages:
  my-local-pkg: ./local-path`

	err := os.WriteFile(filepath.Join(tmpDir, "Pulumi.yaml"), []byte(pulumiYaml), 0o600)
	require.NoError(t, err)

	sink := diagtest.LogSink(t)
	pctx := &plugin.Context{
		Diag: sink,
		Root: tmpDir,
	}

	pluginSpec := workspace.PluginSpec{Name: "my-local-pkg", Kind: apitype.ResourcePlugin}
	descriptor := workspace.PackageDescriptor{PluginSpec: pluginSpec}

	setupCalled := false
	setupProviderFunc := func(
		desc workspace.PackageDescriptor, spec *workspace.PackageSpec,
	) (Provider, *workspace.PackageSpec, error) {
		setupCalled = true
		return Provider{}, spec, nil
	}

	reg := &backend.MockCloudRegistry{
		ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
			return func(yield func(apitype.PackageMetadata, error) bool) {}
		},
	}

	_, _, err = tryRegistryResolution(pctx, reg, pluginSpec, descriptor, setupProviderFunc)

	require.NoError(t, err)
	assert.True(t, setupCalled)
}
