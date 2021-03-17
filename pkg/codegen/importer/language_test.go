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
	"encoding/json"
	"io"
	"io/ioutil"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/internal/test"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/stretchr/testify/assert"
)

func TestGenerateLanguageDefinition(t *testing.T) {
	loader := schema.NewPluginLoader(test.NewHost(testdataPath))

	cases, err := readTestCases("testdata/cases.json")
	if !assert.NoError(t, err) {
		t.Fatal()
	}

	for _, s := range cases.Resources {
		t.Run(string(s.URN), func(t *testing.T) {
			state, err := stack.DeserializeResource(s, config.NopDecrypter, config.NopEncrypter)
			if !assert.NoError(t, err) {
				t.Fatal()
			}

			var actualState *resource.State
			err = GenerateLanguageDefinitions(ioutil.Discard, loader, func(_ io.Writer, p *hcl2.Program) error {
				if !assert.Len(t, p.Nodes, 1) {
					t.Fatal()
				}

				res, isResource := p.Nodes[0].(*hcl2.Resource)
				if !assert.True(t, isResource) {
					t.Fatal()
				}

				actualState = renderResource(t, res)
				return nil
			}, []*resource.State{state}, names)
			if !assert.NoError(t, err) {
				t.Fatal()
			}

			assert.Equal(t, state.Type, actualState.Type)
			assert.Equal(t, state.URN, actualState.URN)
			assert.Equal(t, state.Parent, actualState.Parent)
			assert.Equal(t, state.Provider, actualState.Provider)
			assert.Equal(t, state.Protect, actualState.Protect)
			if !assert.True(t, actualState.Inputs.DeepEquals(state.Inputs)) {
				actual, err := stack.SerializeResource(actualState, config.NopEncrypter, false)
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
