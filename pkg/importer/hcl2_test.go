// Copyright 2016-2020, Pulumi Corporation.
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
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/stretchr/testify/assert"
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

func renderExpr(t *testing.T, x model.Expression) resource.PropertyValue {
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
		return resource.NewNullProperty()
	}
}

func renderLiteralValue(t *testing.T, x *model.LiteralValueExpression) resource.PropertyValue {
	switch x.Value.Type() {
	case cty.Bool:
		return resource.NewBoolProperty(x.Value.True())
	case cty.Number:
		f, _ := x.Value.AsBigFloat().Float64()
		return resource.NewNumberProperty(f)
	case cty.String:
		return resource.NewStringProperty(x.Value.AsString())
	default:
		assert.Failf(t, "", "unexpected literal of type %v", x.Value.Type())
		return resource.NewNullProperty()
	}
}

func renderTemplate(t *testing.T, x *model.TemplateExpression) resource.PropertyValue {
	if len(x.Parts) == 1 {
		return renderLiteralValue(t, x.Parts[0].(*model.LiteralValueExpression))
	}
	b := ""
	for _, p := range x.Parts {
		b += p.(*model.LiteralValueExpression).Value.AsString()
	}
	return resource.NewStringProperty(b)
}

func renderObjectCons(t *testing.T, x *model.ObjectConsExpression) resource.PropertyValue {
	obj := resource.PropertyMap{}
	for _, item := range x.Items {
		kv := renderExpr(t, item.Key)
		if !assert.True(t, kv.IsString()) {
			continue
		}
		obj[resource.PropertyKey(kv.StringValue())] = renderExpr(t, item.Value)
	}
	return resource.NewObjectProperty(obj)
}

func renderScopeTraversal(t *testing.T, x *model.ScopeTraversalExpression) resource.PropertyValue {
	if !assert.Len(t, x.Traversal, 1) {
		return resource.NewNullProperty()
	}

	switch x.RootName {
	case "parent":
		return resource.NewStringProperty(string(parentURN))
	case "provider":
		return resource.NewStringProperty(string(providerURN))
	default:
		assert.Failf(t, "", "unexpected variable reference %v", x.RootName)
		return resource.NewNullProperty()
	}
}

func renderTupleCons(t *testing.T, x *model.TupleConsExpression) resource.PropertyValue {
	arr := make([]resource.PropertyValue, len(x.Expressions))
	for i, x := range x.Expressions {
		arr[i] = renderExpr(t, x)
	}
	return resource.NewArrayProperty(arr)
}

func renderFunctionCall(t *testing.T, x *model.FunctionCallExpression) resource.PropertyValue {
	switch x.Name {
	case "fileArchive":
		if !assert.Len(t, x.Args, 1) {
			return resource.NewNullProperty()
		}
		expr := renderExpr(t, x.Args[0])
		if !assert.True(t, expr.IsString()) {
			return resource.NewNullProperty()
		}
		return resource.NewStringProperty(expr.StringValue())
	case "fileAsset":
		if !assert.Len(t, x.Args, 1) {
			return resource.NewNullProperty()
		}
		expr := renderExpr(t, x.Args[0])
		if !assert.True(t, expr.IsString()) {
			return resource.NewNullProperty()
		}
		return resource.NewStringProperty(expr.StringValue())
	case "secret":
		if !assert.Len(t, x.Args, 1) {
			return resource.NewNullProperty()
		}
		return resource.MakeSecret(renderExpr(t, x.Args[0]))
	default:
		assert.Failf(t, "", "unexpected call to %v", x.Name)
		return resource.NewNullProperty()
	}
}

func renderResource(t *testing.T, r *pcl.Resource) *resource.State {
	inputs := resource.PropertyMap{}
	for _, attr := range r.Inputs {
		inputs[resource.PropertyKey(attr.Name)] = renderExpr(t, attr.Value)
	}

	protect := false
	var parent resource.URN
	var providerRef string
	var importID resource.ID
	if r.Options != nil {
		if r.Options.Protect != nil {
			v, diags := r.Options.Protect.Evaluate(&hcl.EvalContext{})
			if assert.Len(t, diags, 0) && assert.Equal(t, cty.Bool, v.Type()) {
				protect = v.True()
			}
		}
		if r.Options.Parent != nil {
			v := renderExpr(t, r.Options.Parent)
			if assert.True(t, v.IsString()) {
				parent = resource.URN(v.StringValue())
			}
		}
		if r.Options.Provider != nil {
			v := renderExpr(t, r.Options.Provider)
			if assert.True(t, v.IsString()) {
				providerRef = v.StringValue() + "::id"
			}
		}
		if r.Options.ImportID != nil {
			v := renderExpr(t, r.Options.ImportID)
			if assert.True(t, v.IsString()) {
				importID = resource.ID(v.StringValue())
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
		Type:     token,
		URN:      resource.NewURN("stack", "project", parentType, token, r.LogicalName()),
		Custom:   true,
		Inputs:   inputs,
		Parent:   parent,
		Provider: providerRef,
		Protect:  protect,
		ImportID: importID,
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

	loader := schema.NewPluginLoader(utils.NewHost(testdataPath))
	cases, err := readTestCases("testdata/cases.json")
	if !assert.NoError(t, err) {
		t.Fatal()
	}

	//nolint:paralleltest // false positive because range var isn't used directly in t.Run(name) arg
	for _, s := range cases.Resources {
		s := s
		t.Run(string(s.URN), func(t *testing.T) {
			state, err := stack.DeserializeResource(s, config.NopDecrypter)
			if !assert.NoError(t, err) {
				t.Fatal()
			}

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
				Names:    names,
				Snapshot: snapshot,
			}

			block, _, err := GenerateHCL2Definition(loader, state, importState)
			if !assert.NoError(t, err) {
				t.Fatal()
			}

			text := fmt.Sprintf("%v", block)

			parser := syntax.NewParser()
			err = parser.ParseFile(strings.NewReader(text), string(state.URN)+".pp")
			if !assert.NoError(t, err) || !assert.False(t, parser.Diagnostics.HasErrors()) {
				t.Fatal()
			}

			p, diags, err := pcl.BindProgram(parser.Files, pcl.Loader(loader), pcl.AllowMissingVariables)
			assert.NoError(t, err)
			assert.False(t, diags.HasErrors())

			if !assert.Len(t, p.Nodes, 1) {
				t.Fatal()
			}

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
				actual, err := stack.SerializeResource(context.Background(), actualState, config.NopEncrypter, false)
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

// Tests that HCL definitions can be generated even if there is a mismatch in the version of the provider in the
// snapshot and the version of the provider loaded from the plugin.
func TestGenerateHCL2DefinitionsWithVersionMismatches(t *testing.T) {
	t.Parallel()

	// Arrange.
	pkg := tokens.Package("aws")
	requestVersion := "4.37.0"
	loadVersion := "4.37.1"

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
	schemaLoader := schema.NewPluginLoader(host)

	state := &resource.State{
		Type:     "aws:cloudformation/stack:Stack",
		URN:      "urn:pulumi:stack::project::aws:cloudformation/stack:Stack::Stack",
		Custom:   true,
		Provider: "urn:pulumi:stack::project::pulumi:providers:aws::default_123::123",
		Inputs: resource.PropertyMap{
			"name":         resource.NewStringProperty("foobar"),
			"templateBody": resource.NewStringProperty("foobar"),
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
					"version": resource.NewStringProperty("4.37.0"),
				},
			},
		},
	}

	// Act.
	_, _, err := GenerateHCL2Definition(schemaLoader, state, importState)

	// Assert.
	assert.NoError(t, err)
}

func TestGenerateHCL2DefinitionsWithDependantResources(t *testing.T) {
	t.Parallel()
	loader := schema.NewPluginLoader(utils.NewHost(testdataPath))

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
			Inputs: map[string]interface{}{
				// this will be replaced with a reference to exampleBucket.id in the generated code
				"bucket":       "provider-generated-bucket-id-abc123",
				"storageClass": "STANDARD",
			},
			Provider: fmt.Sprintf("%s::%s", snapshot[0].URN, snapshot[0].ID),
		},
	}

	states := make([]*resource.State, 0)
	for _, r := range resources {
		state, err := stack.DeserializeResource(r, config.NopDecrypter)
		if !assert.NoError(t, err) {
			t.Fatal()
		}
		states = append(states, state)
	}

	importState := createImportState(states, snapshot, names)

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

func TestGenerateHCL2DefinitionsWithDependantResourcesUsingNameOrArnProperty(t *testing.T) {
	t.Parallel()
	loader := schema.NewPluginLoader(utils.NewHost(testdataPath))

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
			Outputs: map[string]interface{}{
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
			Inputs: map[string]interface{}{
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
			Inputs: map[string]interface{}{
				// this will be replaced with a reference to exampleBucket.arn in the generated code
				"bucket":       "arn:aws:s3:bucket-12345",
				"storageClass": "STANDARD",
			},
		},
	}

	states := make([]*resource.State, 0)
	for _, r := range resources {
		state, err := stack.DeserializeResource(r, config.NopDecrypter)
		if !assert.NoError(t, err) {
			t.Fatal()
		}
		states = append(states, state)
	}

	importState := createImportState(states, snapshot, names)

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
	loader := schema.NewPluginLoader(utils.NewHost(testdataPath))

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
			Inputs: map[string]interface{}{
				// this will *NOT* be replaced with a reference to either firstBucket.id or secondBucket.id
				// because both have the same ID and it would be ambiguous
				"bucket":       "provider-generated-bucket-id-abc123",
				"storageClass": "STANDARD",
			},
		},
	}

	states := make([]*resource.State, 0)
	for _, r := range resources {
		state, err := stack.DeserializeResource(r, config.NopDecrypter)
		if !assert.NoError(t, err) {
			t.Fatal()
		}
		states = append(states, state)
	}

	importState := createImportState(states, snapshot, names)

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
	loader := schema.NewPluginLoader(utils.NewHost(testdataPath))

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
			Inputs: map[string]interface{}{
				// this literal value will stay as is since it shouldn't self-reference the bucket object itself
				"bucket":       "provider-generated-bucket-object-id-abc123",
				"storageClass": "STANDARD",
			},
		},
	}

	states := make([]*resource.State, 0)
	for _, r := range resources {
		state, err := stack.DeserializeResource(r, config.NopDecrypter)
		if !assert.NoError(t, err) {
			t.Fatal()
		}
		states = append(states, state)
	}

	importState := createImportState(states, snapshot, names)

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

func makeProperty(name string, t schema.Type) *schema.Property {
	return &schema.Property{Name: name, Type: t}
}

func makeObjectType(properties ...*schema.Property) *schema.ObjectType {
	return &schema.ObjectType{Properties: properties}
}

func makeOptionalType(t schema.Type) schema.Type {
	return &schema.OptionalType{ElementType: t}
}

func makeObject(input map[string]resource.PropertyValue) resource.PropertyValue {
	properties := make(map[resource.PropertyKey]resource.PropertyValue)
	for key, value := range input {
		properties[resource.PropertyKey(key)] = value
	}

	return resource.NewObjectProperty(properties)
}

func TestStructuralTypeChecks(t *testing.T) {
	t.Run("String", func(t *testing.T) {
		t.Parallel()
		value := resource.NewStringProperty("foo")
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
		value := resource.NewBoolProperty(true)
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
		value := resource.NewNumberProperty(42)
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
		value := resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewStringProperty("foo"),
			resource.NewStringProperty("bar"),
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
		value := resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewStringProperty("foo"),
			resource.NewNumberProperty(42),
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

		value := makeObject(map[string]resource.PropertyValue{
			"foo": resource.NewStringProperty("foo"),
			"bar": resource.NewNumberProperty(42),
		})

		assert.True(t, valueStructurallyTypedAs(value, makeObjectType(
			makeProperty("foo", schema.StringType),
			makeProperty("bar", schema.NumberType),
		)))

		assert.False(t, valueStructurallyTypedAs(value, makeObjectType(
			makeProperty("foo", schema.StringType),
			makeProperty("bar", schema.StringType),
		)))

		anotherValue := makeObject(map[string]resource.PropertyValue{
			"a": resource.NewStringProperty("A"),
		})

		// property "a" is missing from the type
		assert.False(t, valueStructurallyTypedAs(anotherValue, makeObjectType(
			makeProperty("b", schema.StringType),
		)))

		objectA := makeObject(map[string]resource.PropertyValue{
			"foo": resource.NewStringProperty("foo"),
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
		complexFittingValue := makeObject(map[string]resource.PropertyValue{
			"foo": resource.NewNumberProperty(100),
		})

		assert.True(t, valueStructurallyTypedAs(complexFittingValue, complexUnionOfObjects))
	})
}

func TestReduceUnionTypeEliminatesUnionsBasicCase(t *testing.T) {
	t.Parallel()
	value := resource.NewStringProperty("hello")
	unionTypeA := makeUnionType(schema.StringType, schema.NumberType)
	unionTypeB := makeUnionType(schema.BoolType, schema.StringType)
	reducedA := reduceUnionType(unionTypeA, value)
	reducedB := reduceUnionType(unionTypeB, value)
	assert.Equal(t, schema.StringType, reducedA)
	assert.Equal(t, schema.StringType, reducedB)
}

func TestReduceUnionTypeEliminatesUnionsRecursively(t *testing.T) {
	t.Parallel()
	value := resource.NewStringProperty("hello")
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
	mixedTypeArray := resource.NewArrayProperty([]resource.PropertyValue{
		resource.NewStringProperty("hello"),
		resource.NewNumberProperty(42),
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
