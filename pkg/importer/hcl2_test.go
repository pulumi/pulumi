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

package importer

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/blang/semver"
	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/utils"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/archive"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/asset"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/urn"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

var testdataPath = filepath.Join("..", "codegen", "testing", "test", "testdata")

const (
	parentName   = "parent"
	providerName = "provider"
	logicalName  = "logical"
)

var (
	parentURN   = resource.NewURN("stack", "project", "", "my::parent", "parent")
	providerURN = resource.NewURN("stack", "project", "", providers.MakeProviderType("pkg"), "provider")
	logicalURN  = resource.NewURN("stack", "project", "", "random:index/randomId:RandomId", "strange logical name")
)

var names = NameTable{
	parentURN:   parentName,
	providerURN: providerName,
	logicalURN:  logicalName,
}

func renderExpr(t require.TestingT, x model.Expression) property.Value {
	switch x := x.(type) {
	case *model.LiteralValueExpression:
		return renderLiteralValue(t, x)
	case *model.ScopeTraversalExpression:
		return renderScopeTraversal(t, x)
	case *model.TemplateExpression:
		return renderTemplate(t, x)
	case *model.TupleConsExpression:
		return renderTupleCons(t, x)
	case *model.ObjectConsExpression:
		return renderObjectCons(t, x)
	case *model.FunctionCallExpression:
		return renderFunctionCall(t, x)
	default:
		assert.Failf(t, "", "unexpected expression of type %T", x)
		return property.New(property.Null)
	}
}

func renderLiteralValue(t require.TestingT, x *model.LiteralValueExpression) property.Value {
	if x.Value.IsNull() {
		return property.New(property.Null)
	}
	switch x.Value.Type() {
	case cty.Bool:
		return property.New(x.Value.True())
	case cty.Number:
		f, _ := x.Value.AsBigFloat().Float64()
		return property.New(f)
	case cty.String:
		return property.New(x.Value.AsString())
	default:
		assert.Failf(t, "", "unexpected literal of type %v (value %v)", x.Value.Type().FriendlyName(), x.Value.GoString())
		return property.New(property.Null)
	}
}

func renderTemplate(t require.TestingT, x *model.TemplateExpression) property.Value {
	if len(x.Parts) == 1 {
		return renderLiteralValue(t, x.Parts[0].(*model.LiteralValueExpression))
	}
	var b strings.Builder
	for _, p := range x.Parts {
		b.WriteString(p.(*model.LiteralValueExpression).Value.AsString())
	}
	return property.New(b.String())
}

func renderObjectCons(t require.TestingT, x *model.ObjectConsExpression) property.Value {
	obj := map[string]property.Value{}
	for _, item := range x.Items {
		kv := renderExpr(t, item.Key)
		if !assert.True(t, kv.IsString()) {
			continue
		}
		obj[kv.AsString()] = renderExpr(t, item.Value)
	}
	return property.New(obj)
}

func renderScopeTraversal(t require.TestingT, x *model.ScopeTraversalExpression) property.Value {
	require.Len(t, x.Traversal, 1)

	switch x.RootName {
	case "parent":
		return property.New(string(parentURN))
	case "provider":
		return property.New(string(providerURN))
	default:
		return property.New(x.RootName)
	}
}

func renderTupleCons(t require.TestingT, x *model.TupleConsExpression) property.Value {
	arr := make([]property.Value, len(x.Expressions))
	for i, x := range x.Expressions {
		arr[i] = renderExpr(t, x)
	}
	return property.New(arr)
}

func renderFunctionCall(t require.TestingT, x *model.FunctionCallExpression) property.Value {
	switch x.Name {
	case "fileArchive":
		require.Len(t, x.Args, 1)
		expr := renderExpr(t, x.Args[0])
		if !assert.True(t, expr.IsString()) {
			return property.New(property.Null)
		}
		return expr
	case "fileAsset":
		require.Len(t, x.Args, 1)
		expr := renderExpr(t, x.Args[0])
		if !assert.True(t, expr.IsString()) {
			return property.New(property.Null)
		}
		return expr
	case "stringAsset":
		require.Len(t, x.Args, 1)
		expr := renderExpr(t, x.Args[0])
		if !assert.True(t, expr.IsString()) {
			return property.New(property.Null)
		}
		a, err := asset.FromText(expr.AsString())
		require.NoError(t, err)
		return property.New(a)
	case "remoteAsset":
		require.Len(t, x.Args, 1)
		expr := renderExpr(t, x.Args[0])
		if !assert.True(t, expr.IsString()) {
			return property.New(property.Null)
		}
		a, err := asset.FromURI(expr.AsString())
		require.NoError(t, err)
		return property.New(a)
	case "remoteArchive":
		require.Len(t, x.Args, 1)
		expr := renderExpr(t, x.Args[0])
		if !assert.True(t, expr.IsString()) {
			return property.New(property.Null)
		}
		a, err := archive.FromURI(expr.AsString())
		require.NoError(t, err)
		return property.New(a)
	case "assetArchive":
		require.Len(t, x.Args, 1)
		obj, ok := x.Args[0].(*model.ObjectConsExpression)
		if !assert.True(t, ok, "assetArchive arg must be an object cons") {
			return property.New(property.Null)
		}
		assets := map[string]any{}
		for _, item := range obj.Items {
			kv := renderExpr(t, item.Key)
			if !assert.True(t, kv.IsString()) {
				continue
			}
			v := renderExpr(t, item.Value)
			switch {
			case v.IsAsset():
				assets[kv.AsString()] = v.AsAsset()
			case v.IsArchive():
				assets[kv.AsString()] = v.AsArchive()
			default:
				assert.Failf(t, "", "assetArchive entry %q is %v, want asset or archive", kv.AsString(), v)
			}
		}
		a, err := archive.FromAssets(assets)
		require.NoError(t, err)
		return property.New(a)
	case "secret":
		require.Len(t, x.Args, 1)
		return renderExpr(t, x.Args[0]).WithSecret(true)
	default:
		assert.Failf(t, "", "unexpected call to %v", x.Name)
		return property.New(property.Null)
	}
}

func renderResource(t require.TestingT, r *pcl.Resource) *resource.State {
	inputs := map[string]property.Value{}
	for _, attr := range r.Inputs {
		inputs[attr.Name] = renderExpr(t, attr.Value)
	}

	protect := false
	var parent resource.URN
	var providerRef string
	var importID resource.ID
	var ignoreChanges []string
	if r.Options != nil {
		if r.Options.Protect != nil {
			v, diags := r.Options.Protect.Evaluate(&hcl.EvalContext{})
			require.Len(t, diags, 0)
			require.Equal(t, cty.Bool, v.Type())

			protect = v.True()
		}
		if r.Options.Parent != nil {
			v := renderExpr(t, r.Options.Parent)
			if assert.True(t, v.IsString()) {
				parent = resource.URN(v.AsString())
			}
		}
		if r.Options.Provider != nil {
			v := renderExpr(t, r.Options.Provider)
			if assert.True(t, v.IsString()) {
				providerRef = v.AsString() + "::id"
			}
		}
		if r.Options.ImportID != nil {
			v := renderExpr(t, r.Options.ImportID)
			if assert.True(t, v.IsString()) {
				importID = resource.ID(v.AsString())
			}
		}
		if r.Options.IgnoreChanges != nil {
			v := renderExpr(t, r.Options.IgnoreChanges)
			if assert.True(t, v.IsArray()) {
				for _, item := range v.AsArray().All {
					if assert.True(t, item.IsString()) {
						ignoreChanges = append(ignoreChanges, item.AsString())
					}
				}
			}
		}
	}

	// Pull the raw token from the resource.
	token := tokens.Type(r.Definition.Labels[1])

	var parentType tokens.Type
	if parent != "" {
		parentType = parent.QualifiedType()
	}
	return &resource.State{
		Type:          token,
		URN:           resource.NewURN("stack", "project", parentType, token, r.LogicalName()),
		Custom:        true,
		Inputs:        resource.ToResourcePropertyMap(property.NewMap(inputs)),
		Parent:        parent,
		Provider:      providerRef,
		Protect:       protect,
		ImportID:      importID,
		IgnoreChanges: ignoreChanges,
	}
}

type testCases struct {
	Resources []apitype.ResourceV3 `json:"resources"`
}

func readTestCases(path string) (testCases, error) {
	f, err := os.Open(path)
	if err != nil {
		return testCases{}, err
	}
	defer contract.IgnoreClose(f)

	var cases testCases
	if err = json.NewDecoder(f).Decode(&cases); err != nil {
		return testCases{}, err
	}
	return cases, nil
}

func TestGenerateHCL2Definition(t *testing.T) {
	t.Parallel()

	loader := schema.NewPluginLoader(utils.NewContext(testdataPath))
	cases, err := readTestCases("testdata/cases.json")
	require.NoError(t, err)

	//nolint:paralleltest // false positive because range var isn't used directly in t.Run(name) arg
	for _, s := range cases.Resources {
		t.Run(string(s.URN), func(t *testing.T) {
			state, err := stack.DeserializeResource(s, config.NopDecrypter)
			require.NoError(t, err)

			snapshot := []*resource.State{
				{
					ID:             "123",
					ImportID:       "abc",
					Custom:         true,
					Type:           "pulumi:providers:aws",
					RetainOnDelete: true,
					IgnoreChanges:  []string{"fooIgnore"},
					DeletedWith:    "123",
					URN:            "urn:pulumi:stack::project::pulumi:providers:aws::default_123",
				},
				{
					ID:             "123",
					ImportID:       "abc",
					Custom:         true,
					Type:           "pulumi:providers:random",
					RetainOnDelete: true,
					IgnoreChanges:  []string{"fooIgnore"},
					DeletedWith:    "123",
					URN:            "urn:pulumi:stack::project::pulumi:providers:random::default_123",
				},
				{
					ID:             "id",
					ImportID:       "abc",
					Custom:         true,
					Type:           "pulumi:providers:pkg",
					RetainOnDelete: true,
					IgnoreChanges:  []string{"fooIgnore"},
					DeletedWith:    "123",
					URN:            "urn:pulumi:stack::project::pulumi:providers:pkg::provider",
				},
				// One test that ensures unset values still pass.
				{
					ID:     "id",
					Custom: true,
					Type:   "pulumi:providers:pkg",
					URN:    "urn:pulumi:stack::project::pulumi:providers:pkg::provider",
				},
			}

			importState := ImportState{
				Names:    maps.Clone(names),
				Snapshot: snapshot,
			}

			block, _, err := GenerateHCL2Definition(loader, state, importState)
			require.NoError(t, err)

			text := fmt.Sprintf("%v", block)

			parser := syntax.NewParser()
			err = parser.ParseFile(strings.NewReader(text), string(state.URN)+".pp")
			require.NoError(t, err)
			require.False(t, parser.Diagnostics.HasErrors())

			p, diags, err := pcl.BindProgram(parser.Files, loader, pcl.AllowMissingVariables)
			require.NoError(t, err)
			assert.False(t, diags.HasErrors())

			require.Len(t, p.Nodes, 1)

			res, isResource := p.Nodes[0].(*pcl.Resource)
			if !assert.True(t, isResource) {
				t.Fatal()
			}

			actualState := renderResource(t, res)

			assert.Equal(t, state.Type, actualState.Type)
			assert.Equal(t, state.URN, actualState.URN)
			assert.Equal(t, state.Parent, actualState.Parent)
			assert.Equal(t, state.ImportID, actualState.ImportID)
			assert.Equal(t, state.RetainOnDelete, actualState.RetainOnDelete)
			assert.Equal(t, state.IgnoreChanges, actualState.IgnoreChanges)
			assert.Equal(t, state.DeletedWith, actualState.DeletedWith)
			if !strings.Contains(state.Provider, "::default_") {
				assert.Equal(t, state.Provider, actualState.Provider)
			}
			assert.Equal(t, state.Protect, actualState.Protect)
			if !assert.True(t, actualState.Inputs.DeepEquals(state.Inputs)) {
				actual, err := stack.SerializeResource(t.Context(), actualState, config.NopEncrypter, false)
				contract.IgnoreError(err)

				sb, err := json.MarshalIndent(s, "", "    ")
				contract.IgnoreError(err)

				ab, err := json.MarshalIndent(actual, "", "    ")
				contract.IgnoreError(err)

				t.Logf("%v", text)
				// We know this will fail, but we want the diff
				assert.Equal(t, string(sb), string(ab))
			}
		})
	}
}

// Tests that HCL definitions can be generated even if there is a mismatch in
// the version of the provider in the state.
func TestGenerateHCL2DefinitionWithProviderDeclaration(t *testing.T) {
	t.Parallel()

	// Arrange
	loader := schema.NewPluginLoader(utils.NewContext(testdataPath))

	state := &resource.State{
		ID:       "someProvider",
		Type:     "pulumi:providers:aws",
		Provider: "urn:pulumi:stack::project::pulumi:providers:aws::default_123::123",
		URN:      "urn:pulumi:stack::project::pulumi:providers:aws::default_123",
		Inputs: resource.PropertyMap{
			"region": resource.NewProperty("us-west-2"),
		},
	}

	importState := ImportState{
		Names: nil,
		Snapshot: []*resource.State{
			{
				ID:       "123",
				ImportID: "abc",
				Type:     "pulumi:providers:aws",
				URN:      "urn:pulumi:stack::project::pulumi:providers:aws::default_123",
				Inputs: resource.PropertyMap{
					"region": resource.NewProperty("some-default-value"),
				},
			},
		},
	}

	// Act.
	block, _, err := GenerateHCL2Definition(loader, state, importState)

	// Assert.
	require.NoError(t, err)
	assert.Equal(t, []model.BodyItem{&model.Attribute{
		Name: "region",
		Value: &model.TemplateExpression{
			Parts: []model.Expression{
				&model.LiteralValueExpression{
					Value: cty.StringVal("us-west-2"),
				},
			},
		},
	}}, block.Body.Items, "expected region to be set on custom provider")
}

// Tests that HCL definitions can be generated even if there is a mismatch in the version of the provider in the
// snapshot and the version of the provider loaded from the plugin.
func TestGenerateHCL2DefinitionsWithVersionMismatches(t *testing.T) {
	t.Parallel()

	// Arrange.
	pkg := tokens.Package("aws")
	requestVersion := "4.26.0"
	loadVersion := "5.4.0"

	pluginLoader := deploytest.NewProviderLoader(pkg, semver.MustParse(requestVersion), func() (plugin.Provider, error) {
		return &deploytest.Provider{
			GetSchemaF: func(context.Context, plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
				path := filepath.Join(testdataPath, fmt.Sprintf("%s-%s.json", pkg, loadVersion))
				data, err := os.ReadFile(path)
				if err != nil {
					return plugin.GetSchemaResponse{}, err
				}
				return plugin.GetSchemaResponse{
					Schema: data,
				}, nil
			},
		}, nil
	})

	host := deploytest.NewPluginHost(nil /*sink*/, nil /*statusSink*/, nil /*languageRuntime*/, pluginLoader)
	pctx, err := plugin.NewContextWithHost(t.Context(), nil, nil, host, "", "", nil)
	require.NoError(t, err)
	schemaLoader := schema.NewPluginLoader(pctx)

	state := &resource.State{
		Type:     "aws:cloudformation/stack:Stack",
		URN:      "urn:pulumi:stack::project::aws:cloudformation/stack:Stack::Stack",
		Custom:   true,
		Provider: "urn:pulumi:stack::project::pulumi:providers:aws::default_123::123",
		Inputs: resource.PropertyMap{
			"name":         resource.NewProperty("foobar"),
			"templateBody": resource.NewProperty("foobar"),
		},
	}

	importState := ImportState{
		Names: nil,
		Snapshot: []*resource.State{
			{
				Type:   "pulumi:providers:aws",
				ID:     "123",
				URN:    "urn:pulumi:stack::project::pulumi:providers:aws::default_123",
				Custom: true,
				Inputs: resource.PropertyMap{
					"version": resource.NewProperty("4.26.0"),
				},
			},
		},
	}

	// Act.
	_, _, err = GenerateHCL2Definition(schemaLoader, state, importState)

	// Assert.
	require.NoError(t, err)
}

func TestGenerateHCL2DefinitionsWithDependantResources(t *testing.T) {
	t.Parallel()
	loader := schema.NewPluginLoader(utils.NewContext(testdataPath))

	snapshot := []*resource.State{
		{
			ID:     "123",
			Custom: true,
			Type:   "pulumi:providers:aws",
			URN:    "urn:pulumi:stack::project::pulumi:providers:aws::default_123",
		},
	}

	resources := []apitype.ResourceV3{
		{
			URN:      "urn:pulumi:stack::project::aws:s3/bucket:Bucket::exampleBucket",
			ID:       "provider-generated-bucket-id-abc123",
			Custom:   true,
			Type:     "aws:s3/bucket:Bucket",
			Provider: fmt.Sprintf("%s::%s", snapshot[0].URN, snapshot[0].ID),
		},
		{
			URN:    "urn:pulumi:stack::project::aws:s3/bucketObject:BucketObject::exampleBucketObject",
			ID:     "provider-generated-bucket-object-id-abc123",
			Custom: true,
			Type:   "aws:s3/bucketObject:BucketObject",
			Inputs: map[string]any{
				// this will be replaced with a reference to exampleBucket.id in the generated code
				"bucket":       "provider-generated-bucket-id-abc123",
				"storageClass": "STANDARD",
			},
			Provider: fmt.Sprintf("%s::%s", snapshot[0].URN, snapshot[0].ID),
		},
	}

	states := slice.Prealloc[*resource.State](len(resources))
	for _, r := range resources {
		state, err := stack.DeserializeResource(r, config.NopDecrypter)
		require.NoError(t, err)
		states = append(states, state)
	}

	importState := createImportState(states, snapshot, maps.Clone(names))

	var hcl2Text strings.Builder
	for i, state := range states {
		hcl2Def, _, err := GenerateHCL2Definition(loader, state, importState)
		if err != nil {
			t.Fatal(err)
		}

		pre := ""
		if i > 0 {
			pre = "\n"
		}
		_, err = fmt.Fprintf(&hcl2Text, "%s%v", pre, hcl2Def)
		contract.IgnoreError(err)
	}

	expectedCode := `resource exampleBucket "aws:s3/bucket:Bucket" {

}

resource exampleBucketObject "aws:s3/bucketObject:BucketObject" {
    bucket = exampleBucket.id
    storageClass = "STANDARD"

}
`

	assert.Equal(t, expectedCode, hcl2Text.String(), "Generated HCL2 code does not match expected code")
}

// Testing that the generated HCL code distinguishes between the logical name (imported from the state)
// and the lexical name used to generate the program with its references.
// Also shows that the logical name is emitted in the form of the __logicalName attribute.
func TestGenerateHCL2DefinitionsWithDependantResourcesUsesLexicalNameInGeneratedCode(t *testing.T) {
	t.Parallel()
	loader := schema.NewPluginLoader(utils.NewContext(testdataPath))

	snapshot := []*resource.State{
		{
			ID:     "123",
			Custom: true,
			Type:   "pulumi:providers:aws",
			URN:    "urn:pulumi:stack::project::pulumi:providers:aws::default_123",
		},
	}

	logicalName := "Bucket & Stuff"
	bucketUrn := "urn:pulumi:stack::project::aws:s3/bucket:Bucket::" + logicalName
	nameTable := NameTable{
		urn.URN(bucketUrn): "lexicalName",
	}

	resources := []apitype.ResourceV3{
		{
			URN:      urn.URN(bucketUrn),
			ID:       "provider-generated-bucket-id-abc123",
			Custom:   true,
			Type:     "aws:s3/bucket:Bucket",
			Provider: fmt.Sprintf("%s::%s", snapshot[0].URN, snapshot[0].ID),
		},
		{
			URN:    "urn:pulumi:stack::project::aws:s3/bucketObject:BucketObject::exampleBucketObject",
			ID:     "provider-generated-bucket-object-id-abc123",
			Custom: true,
			Type:   "aws:s3/bucketObject:BucketObject",
			Inputs: map[string]any{
				// this will be replaced with a reference to exampleBucket.id in the generated code
				"bucket":       "provider-generated-bucket-id-abc123",
				"storageClass": "STANDARD",
			},
			Provider: fmt.Sprintf("%s::%s", snapshot[0].URN, snapshot[0].ID),
		},
	}

	states := slice.Prealloc[*resource.State](len(resources))
	for _, r := range resources {
		state, err := stack.DeserializeResource(r, config.NopDecrypter)
		require.NoError(t, err)
		states = append(states, state)
	}

	importState := createImportState(states, snapshot, nameTable)

	var hcl2Text strings.Builder
	for i, state := range states {
		hcl2Def, _, err := GenerateHCL2Definition(loader, state, importState)
		if err != nil {
			t.Fatal(err)
		}

		pre := ""
		if i > 0 {
			pre = "\n"
		}
		_, err = fmt.Fprintf(&hcl2Text, "%s%v", pre, hcl2Def)
		contract.IgnoreError(err)
	}

	expectedCode := `resource lexicalName "aws:s3/bucket:Bucket" {
    __logicalName = "Bucket & Stuff"

}

resource exampleBucketObject "aws:s3/bucketObject:BucketObject" {
    bucket = lexicalName.id
    storageClass = "STANDARD"

}
`

	assert.Equal(t, expectedCode, hcl2Text.String(), "Generated HCL2 code does not match expected code")
}

func TestGenerateHCL2DefinitionsWithDependantResourcesUsingNameOrArnProperty(t *testing.T) {
	t.Parallel()
	loader := schema.NewPluginLoader(utils.NewContext(testdataPath))

	snapshot := []*resource.State{
		{
			ID:     "123",
			Custom: true,
			Type:   "pulumi:providers:aws",
			URN:    "urn:pulumi:stack::project::pulumi:providers:aws::default_123",
		},
	}

	resources := []apitype.ResourceV3{
		{
			Provider: fmt.Sprintf("%s::%s", snapshot[0].URN, snapshot[0].ID),
			URN:      "urn:pulumi:stack::project::aws:s3/bucket:Bucket::exampleBucket",
			ID:       "provider-generated-bucket-id-abc123",
			Custom:   true,
			Type:     "aws:s3/bucket:Bucket",
			Outputs: map[string]any{
				"name": "bucketName-12345",
				"arn":  "arn:aws:s3:bucket-12345",
			},
		},
		{
			Provider: fmt.Sprintf("%s::%s", snapshot[0].URN, snapshot[0].ID),
			URN:      "urn:pulumi:stack::project::aws:s3/bucketObject:BucketObject::exampleBucketObject",
			ID:       "provider-generated-bucket-object-id-abc123",
			Custom:   true,
			Type:     "aws:s3/bucketObject:BucketObject",
			Inputs: map[string]any{
				// this will be replaced with a reference to exampleBucket.name in the generated code
				"bucket":       "bucketName-12345",
				"storageClass": "STANDARD",
			},
		},
		{
			Provider: fmt.Sprintf("%s::%s", snapshot[0].URN, snapshot[0].ID),
			URN:      "urn:pulumi:stack::project::aws:s3/bucketObject:BucketObject::exampleBucketObjectUsingArn",
			ID:       "provider-generated-bucket-object-id-abc123",
			Custom:   true,
			Type:     "aws:s3/bucketObject:BucketObject",
			Inputs: map[string]any{
				// this will be replaced with a reference to exampleBucket.arn in the generated code
				"bucket":       "arn:aws:s3:bucket-12345",
				"storageClass": "STANDARD",
			},
		},
	}

	states := slice.Prealloc[*resource.State](len(resources))
	for _, r := range resources {
		state, err := stack.DeserializeResource(r, config.NopDecrypter)
		require.NoError(t, err)
		states = append(states, state)
	}

	importState := createImportState(states, snapshot, maps.Clone(names))

	var hcl2Text strings.Builder
	for i, state := range states {
		hcl2Def, _, err := GenerateHCL2Definition(loader, state, importState)
		if err != nil {
			t.Fatal(err)
		}

		pre := ""
		if i > 0 {
			pre = "\n"
		}
		_, err = fmt.Fprintf(&hcl2Text, "%s%v", pre, hcl2Def)
		contract.IgnoreError(err)
	}

	expectedCode := `resource exampleBucket "aws:s3/bucket:Bucket" {

}

resource exampleBucketObject "aws:s3/bucketObject:BucketObject" {
    bucket = exampleBucket.name
    storageClass = "STANDARD"

}

resource exampleBucketObjectUsingArn "aws:s3/bucketObject:BucketObject" {
    bucket = exampleBucket.arn
    storageClass = "STANDARD"

}
`

	assert.Equal(t, expectedCode, hcl2Text.String(), "Generated HCL2 code does not match expected code")
}

func TestGenerateHCL2DefinitionsWithAmbiguousReferencesMaintainsLiteralValue(t *testing.T) {
	t.Parallel()
	loader := schema.NewPluginLoader(utils.NewContext(testdataPath))

	snapshot := []*resource.State{
		{
			ID:     "123",
			Custom: true,
			Type:   "pulumi:providers:aws",
			URN:    "urn:pulumi:stack::project::pulumi:providers:aws::default_123",
		},
	}

	resources := []apitype.ResourceV3{
		{
			Provider: fmt.Sprintf("%s::%s", snapshot[0].URN, snapshot[0].ID),
			URN:      "urn:pulumi:stack::project::aws:s3/bucket:Bucket::firstBucket",
			ID:       "provider-generated-bucket-id-abc123",
			Custom:   true,
			Type:     "aws:s3/bucket:Bucket",
		},
		{
			Provider: fmt.Sprintf("%s::%s", snapshot[0].URN, snapshot[0].ID),
			URN:      "urn:pulumi:stack::project::aws:s3/bucket:Bucket::secondBucket",
			ID:       "provider-generated-bucket-id-abc123",
			Custom:   true,
			Type:     "aws:s3/bucket:Bucket",
		},
		{
			Provider: fmt.Sprintf("%s::%s", snapshot[0].URN, snapshot[0].ID),
			URN:      "urn:pulumi:stack::project::aws:s3/bucketObject:BucketObject::exampleBucketObject",
			ID:       "provider-generated-bucket-object-id-abc123",
			Custom:   true,
			Type:     "aws:s3/bucketObject:BucketObject",
			Inputs: map[string]any{
				// this will *NOT* be replaced with a reference to either firstBucket.id or secondBucket.id
				// because both have the same ID and it would be ambiguous
				"bucket":       "provider-generated-bucket-id-abc123",
				"storageClass": "STANDARD",
			},
		},
	}

	states := slice.Prealloc[*resource.State](len(resources))
	for _, r := range resources {
		state, err := stack.DeserializeResource(r, config.NopDecrypter)
		require.NoError(t, err)
		states = append(states, state)
	}

	importState := createImportState(states, snapshot, maps.Clone(names))

	var hcl2Text strings.Builder
	for i, state := range states {
		hcl2Def, _, err := GenerateHCL2Definition(loader, state, importState)
		if err != nil {
			t.Fatal(err)
		}

		pre := ""
		if i > 0 {
			pre = "\n"
		}
		_, err = fmt.Fprintf(&hcl2Text, "%s%v", pre, hcl2Def)
		contract.IgnoreError(err)
	}

	expectedCode := `resource firstBucket "aws:s3/bucket:Bucket" {

}

resource secondBucket "aws:s3/bucket:Bucket" {

}

resource exampleBucketObject "aws:s3/bucketObject:BucketObject" {
    bucket = "provider-generated-bucket-id-abc123"
    storageClass = "STANDARD"

}
`

	assert.Equal(t, expectedCode, hcl2Text.String(), "Generated HCL2 code does not match expected code")
}

func TestGenerateHCL2DefinitionsDoesNotMakeSelfReferences(t *testing.T) {
	t.Parallel()
	loader := schema.NewPluginLoader(utils.NewContext(testdataPath))

	snapshot := []*resource.State{
		{
			ID:     "123",
			Custom: true,
			Type:   "pulumi:providers:aws",
			URN:    "urn:pulumi:stack::project::pulumi:providers:aws::default_123",
		},
	}

	resources := []apitype.ResourceV3{
		{
			Provider: fmt.Sprintf("%s::%s", snapshot[0].URN, snapshot[0].ID),
			URN:      "urn:pulumi:stack::project::aws:s3/bucketObject:BucketObject::exampleBucketObject",
			ID:       "provider-generated-bucket-object-id-abc123",
			Custom:   true,
			Type:     "aws:s3/bucketObject:BucketObject",
			Inputs: map[string]any{
				// this literal value will stay as is since it shouldn't self-reference the bucket object itself
				"bucket":       "provider-generated-bucket-object-id-abc123",
				"storageClass": "STANDARD",
			},
		},
	}

	states := slice.Prealloc[*resource.State](len(resources))
	for _, r := range resources {
		state, err := stack.DeserializeResource(r, config.NopDecrypter)
		require.NoError(t, err)
		states = append(states, state)
	}

	importState := createImportState(states, snapshot, maps.Clone(names))

	var hcl2Text strings.Builder
	for i, state := range states {
		hcl2Def, _, err := GenerateHCL2Definition(loader, state, importState)
		if err != nil {
			t.Fatal(err)
		}

		pre := ""
		if i > 0 {
			pre = "\n"
		}
		_, err = fmt.Fprintf(&hcl2Text, "%s%v", pre, hcl2Def)
		contract.IgnoreError(err)
	}

	expectedCode := `resource exampleBucketObject "aws:s3/bucketObject:BucketObject" {
    bucket = "provider-generated-bucket-object-id-abc123"
    storageClass = "STANDARD"

}
`

	assert.Equal(t, expectedCode, hcl2Text.String(), "Generated HCL2 code does not match expected code")
}

func TestSimplerType(t *testing.T) {
	t.Parallel()

	types := []schema.Type{
		schema.BoolType,
		schema.IntType,
		schema.NumberType,
		schema.StringType,
		schema.AssetType,
		schema.ArchiveType,
		schema.JSONType,
		&schema.ArrayType{ElementType: schema.BoolType},
		&schema.ArrayType{ElementType: schema.IntType},
		&schema.MapType{ElementType: schema.BoolType},
		&schema.MapType{ElementType: schema.IntType},
		&schema.ObjectType{},
		&schema.ObjectType{
			Properties: []*schema.Property{
				{
					Name: "foo",
					Type: schema.BoolType,
				},
			},
		},
		&schema.ObjectType{
			Properties: []*schema.Property{
				{
					Name: "foo",
					Type: schema.IntType,
				},
			},
		},
		&schema.ObjectType{
			Properties: []*schema.Property{
				{
					Name: "foo",
					Type: schema.IntType,
				},
				{
					Name: "bar",
					Type: schema.IntType,
				},
			},
		},
		&schema.UnionType{ElementTypes: []schema.Type{schema.BoolType, schema.IntType}},
		&schema.UnionType{ElementTypes: []schema.Type{schema.IntType, schema.JSONType}},
		&schema.UnionType{ElementTypes: []schema.Type{schema.NumberType, schema.StringType}},
		schema.AnyType,
	}

	assert.True(t, sort.SliceIsSorted(types, func(i, j int) bool {
		return simplerType(types[i], types[j])
	}))
}

func makeUnionType(types ...schema.Type) *schema.UnionType {
	return &schema.UnionType{ElementTypes: types}
}

func makeArrayType(elementType schema.Type) *schema.ArrayType {
	return &schema.ArrayType{ElementType: elementType}
}

func makeMapType(elementType schema.Type) *schema.MapType {
	return &schema.MapType{ElementType: elementType}
}

func makeProperty(name string, t schema.Type) *schema.Property {
	return &schema.Property{Name: name, Type: t}
}

func makeObjectType(properties ...*schema.Property) *schema.ObjectType {
	return &schema.ObjectType{Properties: properties}
}

func makeOptionalType(t schema.Type) schema.Type {
	return &schema.OptionalType{ElementType: t}
}

func makeObject(input map[string]property.Value) property.Value {
	properties := make(map[string]property.Value)
	for key, value := range input {
		properties[key] = value
	}

	return property.New(properties)
}

func TestStructuralTypeChecks(t *testing.T) {
	t.Run("String", func(t *testing.T) {
		t.Parallel()
		value := property.New("foo")
		assert.True(t, valueStructurallyTypedAs(value, schema.StringType))
		assert.True(t, valueStructurallyTypedAs(value, makeUnionType(schema.StringType)))
		assert.True(t, valueStructurallyTypedAs(value, makeUnionType(schema.StringType, schema.NumberType)))
		assert.True(t, valueStructurallyTypedAs(value, makeUnionType(schema.NumberType, schema.StringType)))

		assert.False(t, valueStructurallyTypedAs(value, schema.BoolType))
		assert.False(t, valueStructurallyTypedAs(value, schema.NumberType))
		assert.False(t, valueStructurallyTypedAs(value, makeUnionType(schema.BoolType, schema.NumberType)))
	})

	t.Run("Bool", func(t *testing.T) {
		t.Parallel()
		value := property.New(true)
		assert.True(t, valueStructurallyTypedAs(value, schema.BoolType))
		assert.True(t, valueStructurallyTypedAs(value, makeUnionType(schema.BoolType)))
		assert.True(t, valueStructurallyTypedAs(value, makeUnionType(schema.BoolType, schema.NumberType)))
		assert.True(t, valueStructurallyTypedAs(value, makeUnionType(schema.NumberType, schema.BoolType)))

		assert.False(t, valueStructurallyTypedAs(value, schema.StringType))
		assert.False(t, valueStructurallyTypedAs(value, schema.NumberType))
		assert.False(t, valueStructurallyTypedAs(value, makeUnionType(schema.StringType, schema.NumberType)))
	})

	t.Run("Number", func(t *testing.T) {
		t.Parallel()
		value := property.New(42.0)
		assert.True(t, valueStructurallyTypedAs(value, schema.NumberType))
		assert.True(t, valueStructurallyTypedAs(value, makeUnionType(schema.NumberType)))
		assert.True(t, valueStructurallyTypedAs(value, makeUnionType(schema.NumberType, schema.StringType)))
		assert.True(t, valueStructurallyTypedAs(value, makeUnionType(schema.StringType, schema.NumberType)))

		assert.False(t, valueStructurallyTypedAs(value, schema.StringType))
		assert.False(t, valueStructurallyTypedAs(value, schema.BoolType))
		assert.False(t, valueStructurallyTypedAs(value, makeUnionType(schema.StringType, schema.BoolType)))
	})

	t.Run("Array", func(t *testing.T) {
		t.Parallel()
		value := property.New([]property.Value{
			property.New("foo"),
			property.New("bar"),
		})

		assert.True(t, valueStructurallyTypedAs(value, makeArrayType(schema.StringType)))
		assert.True(t, valueStructurallyTypedAs(value, makeUnionType(makeArrayType(schema.StringType))))
		assert.True(t, valueStructurallyTypedAs(value, makeUnionType(makeArrayType(schema.StringType), schema.NumberType)))
		assert.True(t, valueStructurallyTypedAs(value, makeUnionType(schema.NumberType, makeArrayType(schema.StringType))))
		assert.True(t, valueStructurallyTypedAs(value, makeUnionType(
			makeArrayType(schema.StringType),
			makeArrayType(schema.NumberType))))

		assert.True(t, valueStructurallyTypedAs(value, makeUnionType(
			makeArrayType(schema.NumberType),
			makeArrayType(schema.StringType))))

		assert.False(t, valueStructurallyTypedAs(value, makeArrayType(schema.BoolType)))
		assert.False(t, valueStructurallyTypedAs(value, makeArrayType(schema.NumberType)))
		assert.False(t, valueStructurallyTypedAs(value, makeUnionType(makeArrayType(schema.BoolType), schema.NumberType)))
		assert.False(t, valueStructurallyTypedAs(value, makeUnionType(schema.NumberType, makeArrayType(schema.BoolType))))
	})

	t.Run("ArrayMixedTypes", func(t *testing.T) {
		t.Parallel()
		value := property.New([]property.Value{
			property.New("foo"),
			property.New(42.0),
		})

		// base case: value of type array[union[string, number]]
		assert.True(t, valueStructurallyTypedAs(value, makeArrayType(makeUnionType(schema.StringType, schema.NumberType))))

		assert.False(t, valueStructurallyTypedAs(value, makeArrayType(schema.NumberType)))
		assert.False(t, valueStructurallyTypedAs(value, makeUnionType(makeArrayType(schema.StringType))))
		assert.False(t, valueStructurallyTypedAs(value, makeUnionType(makeArrayType(schema.NumberType))))
		assert.False(t, valueStructurallyTypedAs(value, makeUnionType(makeArrayType(schema.StringType), schema.NumberType)))
		assert.False(t, valueStructurallyTypedAs(value, makeUnionType(schema.NumberType, makeArrayType(schema.StringType))))
	})

	t.Run("Object", func(t *testing.T) {
		t.Parallel()

		value := makeObject(map[string]property.Value{
			"foo": property.New("foo"),
			"bar": property.New(42.0),
		})

		assert.True(t, valueStructurallyTypedAs(value, makeObjectType(
			makeProperty("foo", schema.StringType),
			makeProperty("bar", schema.NumberType),
		)))

		assert.False(t, valueStructurallyTypedAs(value, makeObjectType(
			makeProperty("foo", schema.StringType),
			makeProperty("bar", schema.StringType),
		)))

		anotherValue := makeObject(map[string]property.Value{
			"a": property.New("A"),
		})

		// property "a" is missing from the type
		assert.False(t, valueStructurallyTypedAs(anotherValue, makeObjectType(
			makeProperty("b", schema.StringType),
		)))

		objectA := makeObject(map[string]property.Value{
			"foo": property.New("foo"),
		})

		objectATypeWithRequiredPropertyBar := makeObjectType(
			makeProperty("foo", schema.StringType),
			makeProperty("bar", schema.NumberType))

		// property "bar" is missing from the value
		// but the type requires it to be present
		// so the value is _not_ structurally typed
		assert.False(t, valueStructurallyTypedAs(objectA, objectATypeWithRequiredPropertyBar))

		objectATypeWithOptionalPropertyBar := makeObjectType(
			makeProperty("foo", schema.StringType),
			makeProperty("bar", makeOptionalType(schema.NumberType)))

		// property "bar" is missing from the value, but it is optional
		// so the value is structurally typed just fine
		assert.True(t, valueStructurallyTypedAs(objectA, objectATypeWithOptionalPropertyBar))

		complexUnionOfObjects := makeUnionType(
			makeObjectType(
				makeProperty("foo", schema.StringType),
				makeProperty("bar", schema.NumberType),
			),
			makeObjectType(
				makeProperty("foo", makeUnionType(schema.NumberType, schema.StringType)),
			))

		// fits the second object of the union
		complexFittingValue := makeObject(map[string]property.Value{
			"foo": property.New(100.0),
		})

		assert.True(t, valueStructurallyTypedAs(complexFittingValue, complexUnionOfObjects))
	})

	t.Run("Enum", func(t *testing.T) {
		t.Parallel()

		// An enum is represented on the wire as a value of its underlying
		// primitive type. valueStructurallyTypedAs must look through the enum
		// to its ElementType so a primitive value can be accepted.
		stringEnum := &schema.EnumType{
			Token:       "a:index:S",
			ElementType: schema.StringType,
			Elements:    []*schema.Enum{{Value: "x"}, {Value: "y"}},
		}
		boolEnum := &schema.EnumType{
			Token:       "a:index:B",
			ElementType: schema.BoolType,
			Elements:    []*schema.Enum{{Value: true}, {Value: false}},
		}

		assert.True(t, valueStructurallyTypedAs(property.New("x"), stringEnum))
		assert.True(t, valueStructurallyTypedAs(property.New(false), boolEnum))
		// A bool against a string-typed enum is still rejected.
		assert.False(t, valueStructurallyTypedAs(property.New(true), stringEnum))
		// Enum wrapped in InputType is unwrapped too.
		assert.True(t, valueStructurallyTypedAs(
			property.New(false),
			&schema.InputType{ElementType: boolEnum}))
	})

	t.Run("MissingTypeCases", func(t *testing.T) {
		t.Parallel()

		// schema.JSONType and schema.AnyResourceType share the
		// "pulumi:pulumi:Any" token with schema.AnyType but are distinct
		// singletons. All three accept any value structurally.
		assert.True(t, valueStructurallyTypedAs(property.New(""), schema.JSONType))
		assert.True(t, valueStructurallyTypedAs(property.New(false), schema.JSONType))
		assert.True(t, valueStructurallyTypedAs(property.New(""), schema.AnyResourceType))
		assert.True(t, valueStructurallyTypedAs(property.New(false), schema.AnyResourceType))

		// A property.Map value against *schema.MapType must accept entries
		// whose values match the element type (the previous implementation
		// only handled ObjectType and UnionType for IsMap values).
		stringMap := &schema.MapType{ElementType: schema.StringType}
		assert.True(t, valueStructurallyTypedAs(
			makeObject(map[string]property.Value{"k": property.New("v")}),
			stringMap))
		// Empty maps trivially match.
		assert.True(t, valueStructurallyTypedAs(makeObject(nil), stringMap))
		// Mismatched element type is still rejected.
		assert.False(t, valueStructurallyTypedAs(
			makeObject(map[string]property.Value{"k": property.New(42.0)}),
			stringMap))
	})

	t.Run("UnionOfInputWrappedObjects", func(t *testing.T) {
		t.Parallel()

		// Schema-bound unions wrap their members in *schema.InputType, e.g.
		// Map<Union<Input<Obj>, Input<Archive>>>. valueStructurallyTypedAs
		// strips *schema.InputType from its argument *before* the union is
		// reduced, but reduceUnionType returns the chosen element as-is —
		// typically Input<Obj>. The type switch below must still see the
		// underlying type, so the function must strip the wrapper again
		// after the reduction.
		objectType := makeObjectType(
			makeProperty("foo", schema.StringType),
		)
		union := makeUnionType(
			&schema.InputType{ElementType: objectType},
			&schema.InputType{ElementType: schema.ArchiveType},
		)

		// A map that fits Obj is accepted even though Obj is wrapped in
		// *schema.InputType inside the union.
		assert.True(t, valueStructurallyTypedAs(
			makeObject(map[string]property.Value{"foo": property.New("x")}),
			union))
		// A map that does not fit Obj (unknown property) is still rejected.
		assert.False(t, valueStructurallyTypedAs(
			makeObject(map[string]property.Value{"unknown": property.New("x")}),
			union))
	})

	t.Run("OptionalWrappers", func(t *testing.T) {
		t.Parallel()

		assert.True(t, valueStructurallyTypedAs(
			property.New(false),
			&schema.OptionalType{ElementType: schema.BoolType}))
		assert.True(t, valueStructurallyTypedAs(
			property.New(""),
			&schema.OptionalType{ElementType: &schema.InputType{ElementType: schema.AnyType}}))
		// Optional<Object> with a present, conforming value.
		assert.True(t, valueStructurallyTypedAs(
			makeObject(map[string]property.Value{"foo": property.New("x")}),
			&schema.OptionalType{ElementType: makeObjectType(
				makeProperty("foo", schema.StringType),
			)}))
		// Optional<Object> with a present, non-conforming value still rejected.
		assert.False(t, valueStructurallyTypedAs(
			makeObject(map[string]property.Value{"foo": property.New(42.0)}),
			&schema.OptionalType{ElementType: makeObjectType(
				makeProperty("foo", schema.StringType),
			)}))
	})

	t.Run("OptionalAcceptsNull", func(t *testing.T) {
		t.Parallel()

		assert.True(t, valueStructurallyTypedAs(
			property.New(property.Null),
			&schema.OptionalType{ElementType: schema.StringType}))
		assert.True(t, valueStructurallyTypedAs(
			property.New(property.Null),
			&schema.OptionalType{ElementType: &schema.InputType{ElementType: schema.BoolType}}))
		// A required (non-Optional) T must still reject null.
		assert.False(t, valueStructurallyTypedAs(
			property.New(property.Null),
			schema.StringType))
	})

	t.Run("UnionOfInputWrappedEnums", func(t *testing.T) {
		t.Parallel()

		// Schema-bound unions wrap enum members in *schema.InputType too, e.g.
		// Union<Input<Enum<bool>>, Input<bool>>. reduceUnionType returns the
		// chosen member as-is (Input<Enum<bool>>), so valueStructurallyTypedAs
		// must strip *both* the input wrapper *and* the enum wrapper after the
		// reduction — otherwise the IsBool case never sees BoolType and the
		// value is wrongly rejected.
		boolEnum := &schema.EnumType{
			Token:       "a:index:B",
			ElementType: schema.BoolType,
			Elements:    []*schema.Enum{{Value: true}, {Value: false}},
		}
		stringEnum := &schema.EnumType{
			Token:       "a:index:S",
			ElementType: schema.StringType,
			Elements:    []*schema.Enum{{Value: "x"}, {Value: "y"}},
		}
		assert.True(t, valueStructurallyTypedAs(
			property.New(false),
			makeUnionType(&schema.InputType{ElementType: boolEnum})))
		assert.True(t, valueStructurallyTypedAs(
			property.New(true),
			makeUnionType(&schema.InputType{ElementType: boolEnum}, schema.NumberType)))
		assert.True(t, valueStructurallyTypedAs(
			property.New("x"),
			makeUnionType(&schema.InputType{ElementType: stringEnum})))
		assert.True(t, valueStructurallyTypedAs(
			property.New(1.5),
			makeUnionType(
				&schema.InputType{ElementType: boolEnum},
				&schema.InputType{ElementType: schema.NumberType},
			)))
	})

	t.Run("Archive", func(t *testing.T) {
		t.Parallel()
		value := property.New(&archive.Archive{Sig: archive.ArchiveSig, Assets: map[string]any{}})

		assert.True(t, valueStructurallyTypedAs(value, schema.ArchiveType))
		assert.True(t, valueStructurallyTypedAs(value, schema.AnyType))
		assert.True(t, valueStructurallyTypedAs(value, schema.JSONType))
		assert.True(t, valueStructurallyTypedAs(value, makeUnionType(schema.ArchiveType)))
		assert.True(t, valueStructurallyTypedAs(value, makeUnionType(schema.ArchiveType, schema.StringType)))
		assert.True(t, valueStructurallyTypedAs(value, makeUnionType(schema.StringType, schema.ArchiveType)))
		assert.True(t, valueStructurallyTypedAs(
			property.New(map[string]property.Value{"k": value}),
			makeMapType(schema.ArchiveType)))

		assert.False(t, valueStructurallyTypedAs(value, schema.AssetType))
		assert.False(t, valueStructurallyTypedAs(value, schema.StringType))
		assert.False(t, valueStructurallyTypedAs(value, makeUnionType(schema.AssetType, schema.StringType)))
	})

	t.Run("Asset", func(t *testing.T) {
		t.Parallel()
		a, err := asset.FromText("hello")
		require.NoError(t, err)
		value := property.New(a)

		assert.True(t, valueStructurallyTypedAs(value, schema.AssetType))
		assert.True(t, valueStructurallyTypedAs(value, schema.AnyType))
		assert.True(t, valueStructurallyTypedAs(value, schema.JSONType))
		assert.True(t, valueStructurallyTypedAs(value, makeUnionType(schema.AssetType)))
		assert.True(t, valueStructurallyTypedAs(value, makeUnionType(schema.AssetType, schema.StringType)))
		assert.True(t, valueStructurallyTypedAs(
			property.New([]property.Value{value}),
			makeArrayType(schema.AssetType)))

		assert.False(t, valueStructurallyTypedAs(value, schema.ArchiveType))
		assert.False(t, valueStructurallyTypedAs(value, schema.StringType))
		assert.False(t, valueStructurallyTypedAs(value, makeUnionType(schema.ArchiveType, schema.StringType)))
	})

	t.Run("ResourceReference", func(t *testing.T) {
		t.Parallel()
		value := property.New(property.ResourceReference{
			URN: "urn:pulumi:s::p::pkg:index:R::r",
			ID:  property.New("id"),
		})
		resType := &schema.ResourceType{Token: "pkg:index:R"}

		assert.True(t, valueStructurallyTypedAs(value, resType))
		assert.True(t, valueStructurallyTypedAs(value, schema.AnyResourceType))
		assert.True(t, valueStructurallyTypedAs(value, schema.AnyType))
		assert.True(t, valueStructurallyTypedAs(value, makeUnionType(resType)))
		assert.True(t, valueStructurallyTypedAs(value, makeUnionType(resType, schema.StringType)))

		assert.False(t, valueStructurallyTypedAs(value, schema.StringType))
		assert.False(t, valueStructurallyTypedAs(value, schema.ArchiveType))
	})
}

func TestGenerateValuePreservesProviderDiscriminators(t *testing.T) {
	t.Parallel()

	value := property.New(map[string]property.Value{
		"__type": property.New("PermissionDescriptorGroup"),
		"name":   property.New("root"),
		"items": property.New([]property.Value{
			property.New(map[string]property.Value{
				"__type": property.New("PermissionDescriptorCondition"),
				"name":   property.New("child"),
			}),
		}),
	})

	expr, err := generateValue(
		&schema.MapType{ElementType: schema.AnyType},
		value,
		ImportState{},
		func(string) {},
	)
	require.NoError(t, err)

	formatted := fmt.Sprintf("%v", expr)
	assert.Contains(t, formatted, `"__type" = "PermissionDescriptorGroup"`)
	assert.Contains(t, formatted, `"__type" = "PermissionDescriptorCondition"`)
}

func TestGenerateValueDropsNonConformingMapEntries(t *testing.T) {
	t.Parallel()

	value := property.New(map[string]property.Value{
		"good1": property.New("hello"),
		"good2": property.New("world"),
		"bad1":  property.New(42.0),
		"bad2":  property.New(true),
		"bad3":  property.New(property.Array{}),
	})

	expr, err := generateValue(
		&schema.MapType{ElementType: schema.StringType},
		value,
		ImportState{},
		func(string) {},
	)
	require.NoError(t, err)

	rendered := renderExpr(t, expr)
	require.True(t, rendered.IsMap())

	assert.Equal(t, map[string]property.Value{
		"\"good1\"": property.New("hello"),
		"\"good2\"": property.New("world"),
	}, rendered.AsMap().AsMap())
}

// TestGenerateValueEscapesTemplateMapKeys verifies that map keys containing
// HCL template-interpolation sequences (`${`, `%{`) are escaped so the
// resulting text round-trips through the HCL parser without losing one of
// the leading sigils.
func TestGenerateValueEscapesTemplateMapKeys(t *testing.T) {
	t.Parallel()

	cases := []struct {
		key string
		hcl string
	}{
		{"$${", `"$$${"`},
		{"%{", `"%%{"`},
		{"${foo}", `"$${foo}"`},
		{"plain", `"plain"`},
	}

	for _, c := range cases {
		t.Run(c.key, func(t *testing.T) {
			t.Parallel()
			value := property.New(map[string]property.Value{
				c.key: property.New("v"),
			})
			expr, err := generateValue(
				&schema.MapType{ElementType: schema.StringType},
				value, ImportState{}, func(string) {},
			)
			require.NoError(t, err)
			obj, ok := expr.(*model.ObjectConsExpression)
			require.True(t, ok)
			require.Len(t, obj.Items, 1)
			lit, ok := obj.Items[0].Key.(*model.LiteralValueExpression)
			require.True(t, ok)
			assert.Equal(t, c.hcl, lit.Value.AsString())
		})
	}
}

func TestReduceUnionTypeEliminatesUnionsBasicCase(t *testing.T) {
	t.Parallel()
	value := property.New("hello")
	unionTypeA := makeUnionType(schema.StringType, schema.NumberType)
	unionTypeB := makeUnionType(schema.BoolType, schema.StringType)
	reducedA := reduceUnionType(unionTypeA, value)
	reducedB := reduceUnionType(unionTypeB, value)
	assert.Equal(t, schema.StringType, reducedA)
	assert.Equal(t, schema.StringType, reducedB)
}

func TestReduceUnionTypeEliminatesUnionsRecursively(t *testing.T) {
	t.Parallel()
	value := property.New("hello")
	unionType := makeUnionType(
		makeUnionType(schema.NumberType, schema.BoolType),
		makeUnionType(
			makeUnionType(
				schema.StringType,
				schema.BoolType)))

	reduced := reduceUnionType(unionType, value)
	assert.Equal(t, schema.StringType, reduced)
}

func TestReduceUnionTypeWorksWithArrayOfUnions(t *testing.T) {
	t.Parallel()

	// array[union[string, number]]
	mixedTypeArray := property.New([]property.Value{
		property.New("hello"),
		property.New(42.0),
	})

	// union[array[union[string, number]]]
	arrayType := makeUnionType(&schema.ArrayType{
		ElementType: makeUnionType(schema.StringType, schema.NumberType),
	})

	reduced := reduceUnionType(arrayType, mixedTypeArray)
	schemaArrayType, isArray := reduced.(*schema.ArrayType)
	assert.True(t, isArray)
	unionType, isUnion := schemaArrayType.ElementType.(*schema.UnionType)
	assert.True(t, isUnion)
	assert.Equal(t, schema.StringType, unionType.ElementTypes[0])
	assert.Equal(t, schema.NumberType, unionType.ElementTypes[1])
}
