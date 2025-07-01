// Copyright 2020-2024, Pulumi Corporation.
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

package nodejs

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model/format"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/test"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateProgramVersionSelection(t *testing.T) {
	t.Parallel()

	test.GenerateNodeJSProgramTest(
		t,
		GenerateProgram,
		func(
			directory string, project workspace.Project, program *pcl.Program, localDependencies map[string]string,
		) error {
			return GenerateProject(directory, project, program, localDependencies, false)
		},
	)
}

func TestEnumReferencesCorrectIdentifier(t *testing.T) {
	t.Parallel()
	s := &schema.Package{
		Name: "pulumiservice",
		Language: map[string]interface{}{
			"nodejs": NodePackageInfo{
				PackageName: "@pulumi/bar",
			},
		},
	}
	result, err := enumNameWithPackage("pulumiservice:index:WebhookFilters", s.Reference())
	require.NoError(t, err)
	assert.Equal(t, "pulumiservice.WebhookFilters", result)

	// These are redundant, but serve to clarify our expectations around package alias names.
	assert.NotEqual(t, "bar.WebhookFilters", result)
	assert.NotEqual(t, "@pulumi/bar.WebhookFilters", result)
}

func TestCollectProgramImportsRespectsPackageName(t *testing.T) {
	t.Parallel()

	hcl := `
default = invoke("scaleway:account/getProject:getProject", {})

resource "app" "scaleway:iam/application:Application" {}
`

	scalewaySchema := schema.PackageSpec{
		Name: "scaleway",
		Resources: map[string]schema.ResourceSpec{
			"scaleway:iam/application:Application": {},
		},
		Functions: map[string]schema.FunctionSpec{
			"scaleway:account/getProject:getProject": {},
		},
		Language: map[string]schema.RawMessage{
			"nodejs": schema.RawMessage(`{"packageName": "@pulumiverse/scaleway"}`),
		},
	}

	scalewaySchemaBytes, err := json.MarshalIndent(&scalewaySchema, "", "  ")
	require.NoError(t, err)

	parser := syntax.NewParser()
	err = parser.ParseFile(bytes.NewReader([]byte(hcl)), "infra.tf")
	require.NoError(t, err, "parse failed")
	program, diags, err := pcl.BindProgram(parser.Files, pcl.PluginHost(&plugin.MockHost{
		ResolvePluginF: func(spec workspace.PluginSpec) (*workspace.PluginInfo, error) {
			return &workspace.PluginInfo{Name: spec.Name}, nil
		},
		ProviderF: func(descriptor workspace.PackageDescriptor) (plugin.Provider, error) {
			return &plugin.MockProvider{
				GetSchemaF: func(
					ctx context.Context,
					gsr plugin.GetSchemaRequest,
				) (plugin.GetSchemaResponse, error) {
					return plugin.GetSchemaResponse{Schema: scalewaySchemaBytes}, nil
				},
			}, nil
		},
	}))
	if err != nil || diags.HasErrors() {
		for _, d := range diags {
			t.Logf("%s: %s", d.Summary, d.Detail)
		}
		t.Errorf("BindProgram failed: %v", err)
		t.FailNow()
	}

	packages, err := program.PackageSnapshots()
	require.NoError(t, err)
	for _, p := range packages {
		err := p.ImportLanguages(map[string]schema.Language{"nodejs": Importer})
		require.NoError(t, err)
	}

	g := generator{program: program, Formatter: &format.Formatter{Indent: "  "}}
	imp := g.collectProgramImports(program)

	assert.Equal(t, 1, len(imp.importStatements))
	assert.Equal(t, `import * as scaleway from "@pulumiverse/scaleway";`, imp.importStatements[0])
}
