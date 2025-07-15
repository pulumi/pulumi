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
	"io"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/utils"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/stretchr/testify/assert"
)

func TestGenerateLanguageDefinition(t *testing.T) {
	t.Parallel()
	loader := schema.NewPluginLoader(utils.NewHost(testdataPath))

	cases, err := readTestCases("testdata/cases.json")
	require.NoError(t, err)

	//nolint:paralleltest // false positive because range var isn't used directly in t.Run(name) arg
	for _, s := range cases.Resources {
		s := s
		t.Run(string(s.URN), func(t *testing.T) {
			t.Parallel()
			state, err := stack.DeserializeResource(s, config.NopDecrypter)
			require.NoError(t, err)

			snapshot := []*resource.State{
				{
					ID:     "123",
					Custom: true,
					Type:   "pulumi:providers:aws",
					URN:    "urn:pulumi:stack::project::pulumi:providers:aws::default_123",
				},
				{
					ID:     "123",
					Custom: true,
					Type:   "pulumi:providers:random",
					URN:    "urn:pulumi:stack::project::pulumi:providers:random::default_123",
				},
				{
					ID:     "id",
					Custom: true,
					Type:   "pulumi:providers:pkg",
					URN:    "urn:pulumi:stack::project::pulumi:providers:pkg::provider",
				},
			}

			var actualState *resource.State
			err = GenerateLanguageDefinitions(io.Discard, loader, func(_ io.Writer, p *pcl.Program) error {
				if !assert.Len(t, p.Nodes, 1) {
					t.Fatal()
				}

				res, isResource := p.Nodes[0].(*pcl.Resource)
				if !assert.True(t, isResource) {
					t.Fatal()
				}

				actualState = renderResource(t, res)
				return nil
			}, []*resource.State{state}, snapshot, names)
			require.NoError(t, err)

			assert.Equal(t, state.Type, actualState.Type)
			assert.Equal(t, state.URN, actualState.URN)
			assert.Equal(t, state.Parent, actualState.Parent)
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

				t.Logf("%v\n\n%v\n", string(sb), string(ab))
			}
		})
	}
}

func TestGenerateLanguageDefinitionsRetriesCodegenWhenEncounteringCircularReferences(t *testing.T) {
	t.Parallel()
	loader := schema.NewPluginLoader(utils.NewHost(testdataPath))

	generatedProgram := ""
	generator := func(_ io.Writer, p *pcl.Program) error {
		for _, content := range p.Source() {
			generatedProgram += content
		}
		return nil
	}

	snapshot := []*resource.State{
		{
			ID:     "123",
			Custom: true,
			Type:   "pulumi:providers:aws",
			URN:    "urn:pulumi:stack::project::pulumi:providers:aws::default_123",
		},
	}

	// Create a circular reference between two resources.
	// In this case, generating the PCL would fail with a circular reference error but we retry the codegen
	// without guessing the dependencies between the resources.
	resources := []apitype.ResourceV3{
		{
			URN:    "urn:pulumi:stack::project::aws:s3/bucketObject:BucketObject::first",
			ID:     "bucket-object-1",
			Custom: true,
			Type:   "aws:s3/bucketObject:BucketObject",
			Inputs: map[string]interface{}{
				"bucket": "bucket-object-2",
			},
			Provider: fmt.Sprintf("%s::%s", snapshot[0].URN, snapshot[0].ID),
		},
		{
			URN:    "urn:pulumi:stack::project::aws:s3/bucketObject:BucketObject::second",
			ID:     "bucket-object-2",
			Custom: true,
			Type:   "aws:s3/bucketObject:BucketObject",
			Inputs: map[string]interface{}{
				"bucket": "bucket-object-1",
			},
			Provider: fmt.Sprintf("%s::%s", snapshot[0].URN, snapshot[0].ID),
		},
	}

	states := make([]*resource.State, 0)
	for _, r := range resources {
		state, err := stack.DeserializeResource(r, config.NopDecrypter)
		require.NoError(t, err)
		states = append(states, state)
	}

	var names NameTable
	err := GenerateLanguageDefinitions(io.Discard, loader, generator, states, snapshot, names)
	require.NoError(t, err)
	// notice here the generated program doesn't have any references because
	// we retried the codegen without guessing the dependencies between the resources.
	expectedCode := `package aws {
    baseProviderName = "aws"

}

resource first "aws:s3/bucketObject:BucketObject" {
    bucket = "bucket-object-2"

}

resource second "aws:s3/bucketObject:BucketObject" {
    bucket = "bucket-object-1"

}
`
	assert.Equal(t, expectedCode, generatedProgram)
}

func TestGenerateLanguageDefinitionsAllowsGeneratingParentVariables(t *testing.T) {
	t.Parallel()

	loader := schema.NewPluginLoader(utils.NewHost(testdataPath))

	generatedProgram := ""
	generator := func(_ io.Writer, p *pcl.Program) error {
		for _, content := range p.Source() {
			generatedProgram += content
		}
		return nil
	}

	componentURN := resource.NewURN("dev", "project", "", "example:index:MyComponent", "example")
	childURN := resource.NewURN(
		"dev",
		"project",
		"example:index:MyComponent",
		"random:index/randomPet:RandomPet",
		"randomPet")

	nameTable := NameTable{
		componentURN: "parentComponent",
	}

	snapshot := []*resource.State{
		{
			ID:     "123",
			Custom: true,
			Type:   "pulumi:providers:random",
			URN:    "urn:pulumi:stack::project::pulumi:providers:random::default_123",
		},
	}

	resources := []apitype.ResourceV3{
		{
			URN:      childURN,
			Custom:   true,
			Type:     "random:index/randomPet:RandomPet",
			Parent:   componentURN,
			Provider: fmt.Sprintf("%s::%s", snapshot[0].URN, snapshot[0].ID),
		},
	}

	states := make([]*resource.State, 0)
	for _, r := range resources {
		state, err := stack.DeserializeResource(r, config.NopDecrypter)
		require.NoError(t, err)
		states = append(states, state)
	}

	err := GenerateLanguageDefinitions(io.Discard, loader, generator, states, snapshot, nameTable)
	require.NoError(t, err)
	expectedCode := `package random {
    baseProviderName = "random"

}

resource randomPet "random:index/randomPet:RandomPet" {
options {
parent = parentComponent

}

}
`
	assert.Equal(t, expectedCode, generatedProgram)
}
