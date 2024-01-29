// Copyright 2023, Pulumi Corporation.
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

package ast

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/pulumi/esc/syntax"
	"github.com/pulumi/esc/syntax/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

func TestExample(t *testing.T) {
	t.Parallel()

	const example = `
imports:
  - green-channel
  - us-west-2
config:
  aws:
    fn::open:
      provider: aws-oidc
      inputs:
        sessionName: site-prod-session
        roleArn: some-role-arn
  pulumi:
    aws:defaultTags:
      tags:
        environment: prod
`

	syntax, diags := encoding.DecodeYAML("<stdin>", yaml.NewDecoder(strings.NewReader(example)), nil)
	require.Len(t, diags, 0)

	environment, diags := ParseEnvironment([]byte(example), syntax)
	assert.Len(t, diags, 0)

	assert.Nil(t, environment.Description)
}

func TestExample2(t *testing.T) {
	t.Parallel()

	const example = `
imports:
  - green-channel
  - us-west-2
config:
  aws:
    fn::open::aws-oidc:
      sessionName: site-prod-session
      roleArn: some-role-arn
  pulumi:
    aws:defaultTags:
      tags:
        environment: prod
`

	syntax, diags := encoding.DecodeYAML("<stdin>", yaml.NewDecoder(strings.NewReader(example)), nil)
	require.Len(t, diags, 0)

	environment, diags := ParseEnvironment([]byte(example), syntax)
	assert.Len(t, diags, 0)

	assert.Nil(t, environment.Description)
}

func TestEmptyDocument(t *testing.T) {
	t.Parallel()

	const example = ``

	syntax, diags := encoding.DecodeYAML("<stdin>", yaml.NewDecoder(strings.NewReader(example)), nil)
	require.Len(t, diags, 0)

	environment, diags := ParseEnvironment([]byte(example), syntax)
	assert.Len(t, diags, 0)

	assert.Nil(t, environment.Description)
}

func accept() bool {
	return cmdutil.IsTruthy(os.Getenv("PULUMI_ACCEPT"))
}

func sortEnvironmentDiagnostics(diags syntax.Diagnostics) {
	sort.Slice(diags, func(i, j int) bool {
		di, dj := diags[i], diags[j]
		if di.Subject == nil {
			if dj.Subject == nil {
				return di.Summary < dj.Summary
			}
			return true
		}
		if dj.Subject == nil {
			return false
		}
		if di.Subject.Filename != dj.Subject.Filename {
			return di.Subject.Filename < dj.Subject.Filename
		}
		if di.Subject.Start.Line != dj.Subject.Start.Line {
			return di.Subject.Start.Line < dj.Subject.Start.Line
		}
		return di.Subject.Start.Column < dj.Subject.Start.Column
	})
}

type tagDecoder int

func (d tagDecoder) DecodeTag(filename string, n *yaml.Node) (syntax.Node, syntax.Diagnostics, bool) {
	return nil, nil, false
}

// loadYAMLBytes decodes a YAML template from a byte array.
func loadYAMLBytes(filename string, source []byte) (*EnvironmentDecl, syntax.Diagnostics, error) {
	var diags syntax.Diagnostics

	syn, sdiags := encoding.DecodeYAMLBytes(filename, source, tagDecoder(0))
	diags.Extend(sdiags...)
	if sdiags.HasErrors() {
		return nil, diags, nil
	}

	t, tdiags := ParseEnvironment(source, syn)
	diags.Extend(tdiags...)
	return t, diags, nil
}

func TestParse(t *testing.T) {
	type expectedData struct {
		Decl  any                `json:"decl,omitempty"`
		Diags syntax.Diagnostics `json:"diags,omitempty"`
	}

	path := filepath.Join("testdata", "parse")
	entries, err := os.ReadDir(path)
	require.NoError(t, err)
	for _, e := range entries {
		t.Run(e.Name(), func(t *testing.T) {
			basePath := filepath.Join(path, e.Name())
			envPath, expectedPath := filepath.Join(basePath, "env.yaml"), filepath.Join(basePath, "expected.json")

			envBytes, err := os.ReadFile(envPath)
			require.NoError(t, err)

			decl, diags, err := loadYAMLBytes(e.Name(), envBytes)
			require.NoError(t, err)
			sortEnvironmentDiagnostics(diags)

			if accept() {
				bytes, err := json.MarshalIndent(expectedData{
					Decl:  decl,
					Diags: diags,
				}, "", "    ")
				bytes = append(bytes, '\n')
				require.NoError(t, err)

				err = os.WriteFile(expectedPath, bytes, 0600)
				require.NoError(t, err)

				return
			}

			var expected expectedData
			expectedBytes, err := os.ReadFile(expectedPath)
			require.NoError(t, err)
			dec := json.NewDecoder(bytes.NewReader(expectedBytes))
			dec.UseNumber()
			err = dec.Decode(&expected)
			require.NoError(t, err)

			assert.Equal(t, expected.Diags, diags)

			declJSON, err := json.MarshalIndent(decl, "", "    ")
			require.NoError(t, err)
			var roundTripDecl any
			err = json.Unmarshal(declJSON, &roundTripDecl)
			require.NoError(t, err)
			declJSON, err = json.MarshalIndent(roundTripDecl, "", "    ")
			require.NoError(t, err)

			expectedJSON, err := json.MarshalIndent(expected.Decl, "", "    ")
			require.NoError(t, err)

			assert.Equal(t, string(expectedJSON), string(declJSON))
		})
	}
}
