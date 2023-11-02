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

package encoding

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/esc/syntax"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func accept() bool {
	return cmdutil.IsTruthy(os.Getenv("PULUMI_ACCEPT"))
}

func sortDiagnostics(diags syntax.Diagnostics) {
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

func TestYAML(t *testing.T) {
	type expectedData struct {
		Syntax      *Node              `json:"syntax,omitempty"`
		Diags       syntax.Diagnostics `json:"diags,omitempty"`
		EncodeDiags syntax.Diagnostics `json:"encodeDiags,omitempty"`
	}

	path := filepath.Join("testdata", "yaml")
	entries, err := os.ReadDir(path)
	require.NoError(t, err)
	for _, e := range entries {
		t.Run(e.Name(), func(t *testing.T) {
			basepath := filepath.Join(path, e.Name())
			yamlpath := filepath.Join(basepath, "doc.yaml")
			decodedPath := filepath.Join(basepath, "decoded.json")
			encodedPath := filepath.Join(basepath, "encoded.yaml")

			yamlBytes, err := os.ReadFile(yamlpath)
			require.NoError(t, err)

			root, diags := DecodeYAMLBytes(e.Name(), yamlBytes, nil)
			sortDiagnostics(diags)

			var syn *Node
			var encoded []byte
			var encodeDiags syntax.Diagnostics
			if root != nil {
				s := NewNode(root)
				syn = &s

				var b bytes.Buffer
				enc := yaml.NewEncoder(&b)
				enc.SetIndent(2)

				encodeDiags = EncodeYAML(enc, root)
				encoded = b.Bytes()
			}

			if accept() {
				bytes, err := json.MarshalIndent(expectedData{
					Syntax:      syn,
					Diags:       diags,
					EncodeDiags: encodeDiags,
				}, "", "    ")
				require.NoError(t, err)

				err = os.WriteFile(decodedPath, bytes, 0o600)
				require.NoError(t, err)

				if len(encoded) != 0 {
					err = os.WriteFile(encodedPath, encoded, 0o600)
					require.NoError(t, err)
				}

				return
			}

			var expected expectedData
			expectedBytes, err := os.ReadFile(decodedPath)
			require.NoError(t, err)
			dec := json.NewDecoder(bytes.NewReader(expectedBytes))
			dec.UseNumber()
			err = dec.Decode(&expected)
			require.NoError(t, err)

			var expectedYAML []byte
			if root != nil {
				b, err := os.ReadFile(encodedPath)
				require.NoError(t, err)
				expectedYAML = b
			}

			assert.Equal(t, expected.Syntax, syn)
			assert.Equal(t, expected.Diags, diags)
			assert.Equal(t, expectedYAML, encoded)
			assert.Equal(t, encodeDiags, expected.EncodeDiags)
		})
	}
}

type comment string

func (comment) Range() *hcl.Range {
	return nil
}

func (comment) Path() string {
	return ""
}

func (c comment) HeadComment() string { return string(c) }
func (c comment) LineComment() string { return "" }
func (c comment) FootComment() string { return "" }

func TestYAMLEdit(t *testing.T) {
	const doc = `foo: bar # line comment
baz: qux
`

	const expected = `foo: 42 # line comment
# head comment
baz: qux
`

	rootNode, diags := DecodeYAML("yaml", yaml.NewDecoder(strings.NewReader(doc)), nil)
	assert.Empty(t, diags)

	root := rootNode.(*syntax.ObjectNode)

	root.SetIndex(0, syntax.ObjectProperty(syntax.String("foo"), syntax.NumberSyntax(root.Index(0).Value.Syntax(), 42)))
	root.SetIndex(1, syntax.ObjectProperty(syntax.StringSyntax(comment("head comment"), "baz"), syntax.String("qux")))

	var b bytes.Buffer
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	diags = EncodeYAML(enc, root)

	assert.Empty(t, diags)
	assert.Equal(t, expected, b.String())
}
