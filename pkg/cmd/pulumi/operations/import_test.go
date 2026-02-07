// Copyright 2016-2023, Pulumi Corporation.
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

package operations

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/importer"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseImportFile_errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc     string
		give     importFile
		wantErrs []string
	}{
		{
			desc: "missing everything",
			give: importFile{Resources: []importSpec{{}}},
			wantErrs: []string{
				"3 errors occurred",
				"resource 0 has no type",
				"resource 0 has no name",
				"resource 0 has no ID",
			},
		},
		{
			desc: "missing name and type",
			give: importFile{
				Resources: []importSpec{
					{ID: "thing"},
				},
			},
			wantErrs: []string{
				"2 errors occurred",
				"resource 'thing' has no type",
				"resource 'thing' has no name",
			},
		},
		{
			desc: "missing ID and type",
			give: importFile{
				Resources: []importSpec{
					{Name: "foo"},
				},
			},
			wantErrs: []string{
				"2 errors occurred",
				"resource 'foo' has no type",
				"resource 'foo' has no ID",
			},
		},
		{
			desc: "missing type",
			give: importFile{
				Resources: []importSpec{
					{
						Name: "foo",
						ID:   "bar",
					},
				},
			},
			wantErrs: []string{
				"1 error occurred",
				"resource 'foo' has no type",
			},
		},
		{
			desc: "missing name",
			give: importFile{
				Resources: []importSpec{
					{
						ID:   "bar",
						Type: "foo:bar:baz",
					},
				},
			},
			wantErrs: []string{
				"1 error occurred",
				"resource 'bar' of type 'foo:bar:baz' has no name",
			},
		},
		{
			desc: "missing id",
			give: importFile{
				Resources: []importSpec{
					{
						Name: "foo",
						Type: "foo:bar:baz",
					},
				},
			},
			wantErrs: []string{
				"1 error occurred",
				"resource 'foo' of type 'foo:bar:baz' has no ID",
			},
		},
		{
			desc: "missing parent",
			give: importFile{
				Resources: []importSpec{
					{
						Name:   "thing",
						ID:     "thing",
						Parent: "unknown",
						Type:   "foo:bar:baz",
					},
				},
			},
			wantErrs: []string{
				"1 error occurred",
				"the parent 'unknown' for resource 'thing' of type 'foo:bar:baz' has no entry in 'nameTable'",
			},
		},
		{
			desc: "missing provider",
			give: importFile{
				Resources: []importSpec{
					{
						Name:     "thing",
						ID:       "thing",
						Provider: "unknown",
						Type:     "foo:bar:baz",
					},
				},
			},
			wantErrs: []string{
				"1 error occurred",
				"the provider 'unknown' for resource 'thing' of type 'foo:bar:baz' has no entry in 'nameTable'",
			},
		},
		{
			desc: "bad version",
			give: importFile{
				Resources: []importSpec{
					{
						Name:    "thing",
						ID:      "thing",
						Type:    "foo:bar:baz",
						Version: "not-a-semver",
					},
				},
			},
			wantErrs: []string{
				"1 error occurred",
				"could not parse version 'not-a-semver' for resource 'thing' of type 'foo:bar:baz'",
			},
		},
		{
			desc: "ambiguous parent",
			give: importFile{
				Resources: []importSpec{
					{
						Name:    "res",
						ID:      "res",
						Type:    "foo:bar:bar",
						Version: "0.0.0",
					},
					{
						Name:    "res",
						ID:      "res",
						Type:    "foo:bar:baz",
						Version: "0.0.0",
					},
					{
						Name:    "res-2",
						ID:      "res-2",
						Type:    "foo:bar:a",
						Parent:  "res",
						Version: "0.0.0",
					},
				},
			},
			wantErrs: []string{
				"resource 'res-2' of type 'foo:bar:a' has an ambiguous parent",
			},
		},
		{
			desc: "ambiguous provider",
			give: importFile{
				NameTable: map[string]resource.URN{
					"res": "whatever",
				},
				Resources: []importSpec{
					{
						Name:    "res",
						ID:      "res",
						Type:    "foo:bar:bar",
						Version: "0.0.0",
					},
					{
						Name:    "res",
						ID:      "res",
						Type:    "foo:bar:baz",
						Version: "0.0.0",
					},
					{
						Name:     "res-2",
						ID:       "res-2",
						Type:     "foo:bar:a",
						Provider: "res",
						Version:  "0.0.0",
					},
				},
			},
			wantErrs: []string{
				"resource 'res-2' of type 'foo:bar:a' has an ambiguous provider",
			},
		},
		{
			desc: "component with ID",
			give: importFile{Resources: []importSpec{{
				Name:      "comp",
				ID:        "some-id",
				Type:      "foo:bar:baz",
				Component: true,
			}}},
			wantErrs: []string{
				"1 error occurred",
				"resource 'comp' of type 'foo:bar:baz' has an ID, but is marked as a component",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			require.NotEmpty(t, tt.wantErrs, "invalid test: wantErrs must not be empty")

			_, _, err := parseImportFile(tt.give, tokens.MustParseStackName("stack"), "proj", false)
			require.Error(t, err)
			for _, wantErr := range tt.wantErrs {
				assert.ErrorContains(t, err, wantErr)
			}
		})
	}
}

func TestParseImportFileJustLogicalName(t *testing.T) {
	t.Parallel()
	f := importFile{
		Resources: []importSpec{
			{
				LogicalName: "thing",
				ID:          "thing",
				Type:        "foo:bar:bar",
			},
		},
	}
	imports, names, err := parseImportFile(f, tokens.MustParseStackName("stack"), "proj", false)
	require.NoError(t, err)
	assert.Equal(t, []deploy.Import{
		{
			Type: "foo:bar:bar",
			Name: "thing",
			ID:   "thing",
		},
	}, imports)
	assert.Equal(t, importer.NameTable{
		"urn:pulumi:stack::proj::foo:bar:bar::thing": "thing",
	}, names)
}

func TestParseImportFileLogicalName(t *testing.T) {
	t.Parallel()
	f := importFile{
		Resources: []importSpec{
			{
				Name:        "thing",
				LogicalName: "different logical name",
				ID:          "thing",
				Type:        "foo:bar:bar",
				Version:     "0.0.0",
			},
		},
	}
	imports, names, err := parseImportFile(f, tokens.MustParseStackName("stack"), "proj", false)
	require.NoError(t, err)
	v := semver.MustParse("0.0.0")
	assert.Equal(t, []deploy.Import{
		{
			Type:    "foo:bar:bar",
			Name:    "different logical name",
			ID:      "thing",
			Version: &v,
		},
	}, imports)
	assert.Equal(t, importer.NameTable{
		"urn:pulumi:stack::proj::foo:bar:bar::different logical name": "thing",
	}, names)
}

func TestParseImportFileSameName(t *testing.T) {
	t.Parallel()
	f := importFile{
		Resources: []importSpec{
			{
				Name:    "thing",
				ID:      "thing",
				Type:    "foo:bar:bar",
				Version: "0.0.0",
			},
			{
				Name:    "thing",
				ID:      "thing",
				Type:    "foo:bar:bar",
				Version: "0.0.0",
			},
		},
	}
	_, _, err := parseImportFile(f, tokens.MustParseStackName("stack"), "proj", false)
	assert.ErrorContains(t, err,
		"resource 'thing' of type 'foo:bar:bar' has an ambiguous URN, set name (or logical name) to be unique")
}

// shows that we disambiguate duplicate resource names by their resource type
func TestParseImportFileSameNameDifferentType(t *testing.T) {
	t.Parallel()
	f := importFile{
		Resources: []importSpec{
			{
				Name: "thing",
				ID:   "thing",
				Type: "foo:bar:bar",
			},
			{
				Name: "thing",
				ID:   "thing",
				Type: "foo:bar:baz",
			},
			{
				Name: "thing",
				ID:   "thing",
				Type: "foo:moo:baz",
			},
		},
	}
	imports, names, err := parseImportFile(f, tokens.MustParseStackName("stack"), "proj", false)
	require.NoError(t, err)
	assert.Equal(t, []deploy.Import{
		{
			Type: "foo:bar:bar",
			Name: "thing",
			ID:   "thing",
		},
		{
			Type: "foo:bar:baz",
			Name: "thing",
			ID:   "thing",
		},
		{
			Type: "foo:moo:baz",
			Name: "thing",
			ID:   "thing",
		},
	}, imports)
	assert.Equal(t, importer.NameTable{
		"urn:pulumi:stack::proj::foo:bar:bar::thing": "thing",
		"urn:pulumi:stack::proj::foo:bar:baz::thing": "thingBaz",
		"urn:pulumi:stack::proj::foo:moo:baz::thing": "thingBaz2",
	}, names)
}

// Test that if we're using the name for another resource in the import file for a parent that we don't need
// to add it to the nameTable and just auto fill in the URN.
func TestParseImportFileAutoURN(t *testing.T) {
	t.Parallel()
	f := importFile{
		Resources: []importSpec{
			// Out of order to test that parseImportFile doesn't require the parent to be defined first.
			{
				Name:   "lastThing",
				ID:     "lastThing",
				Type:   "foo:bar:a",
				Parent: "otherThing",
			},
			{
				Name:      "thing",
				Component: true,
				Type:      "foo:bar:a",
			},
			{
				Name:   "otherThing",
				ID:     "otherThing",
				Type:   "foo:bar:a",
				Parent: "thing",
			},
		},
	}
	imports, nt, err := parseImportFile(f, tokens.MustParseStackName("stack"), "proj", false)
	require.NoError(t, err)

	// Check the parent URN was auto filled in.
	assert.Equal(t, resource.URN("urn:pulumi:stack::proj::foo:bar:a$foo:bar:a::otherThing"), imports[0].Parent)
	assert.Equal(t, resource.URN(""), imports[1].Parent)
	assert.Equal(t, resource.URN("urn:pulumi:stack::proj::foo:bar:a::thing"), imports[2].Parent)

	// Check the nameTable was filled in.
	assert.Equal(t, "otherThing", nt[imports[0].Parent])
	assert.Equal(t, "thing", nt[imports[2].Parent])
}

// Small test to ensure that importFile is marshalled to JSON sensibly, mostly checking that optional fields
// don't show up.
func TestImportFileMarshal(t *testing.T) {
	t.Parallel()

	t.Run("initial", func(t *testing.T) {
		t.Parallel()

		importFile := importFile{
			NameTable: map[string]resource.URN{
				"foo": "urn:pulumi:stack::proj::foo:bar:a::arb",
			},
			Resources: []importSpec{
				{
					Name: "bar",
					Type: "foo:bar:b",
					ID:   "123",
				},
				{
					Name:      "comp",
					Type:      "some/comp",
					Component: true,
				},
				{
					Name:              "thirdParty",
					Type:              "some:third:party",
					ID:                "abc123",
					Parent:            "bar",
					Version:           "1.2.3",
					PluginDownloadURL: "https://example.com",
				},
			},
		}

		expected := `{
  "nameTable": {
    "foo": "urn:pulumi:stack::proj::foo:bar:a::arb"
  },
  "resources": [
    {
      "type": "foo:bar:b",
      "name": "bar",
      "id": "123"
    },
    {
      "type": "some/comp",
      "name": "comp",
      "component": true
    },
    {
      "type": "some:third:party",
      "name": "thirdParty",
      "id": "abc123",
      "parent": "bar",
      "version": "1.2.3",
      "pluginDownloadUrl": "https://example.com"
    }
  ]
}
`
		var buffer bytes.Buffer
		enc := json.NewEncoder(&buffer)
		enc.SetIndent("", "  ")
		err := enc.Encode(importFile)
		require.NoError(t, err)
		assert.Equal(t, expected, buffer.String())
	})

	t.Run("omit empty resources list", func(t *testing.T) {
		t.Parallel()

		importFile := importFile{}

		var buffer bytes.Buffer
		enc := json.NewEncoder(&buffer)
		err := enc.Encode(importFile)
		require.NoError(t, err)
		assert.NotContains(t, buffer.String(), "resources")
	})
}

func TestImportResultJSON(t *testing.T) {
	t.Parallel()

	t.Run("basic result", func(t *testing.T) {
		t.Parallel()

		result := importResult{
			Version: 1,
			Summary: display.ResourceChanges{"import": 2, "same": 1},
			Resources: []importedResource{
				{
					URN:       "urn:pulumi:dev::proj::aws:s3/bucket:Bucket::test",
					Type:      "aws:s3/bucket:Bucket",
					Name:      "test",
					ID:        "bucket-id",
					Operation: "import",
					Protected: true,
				},
			},
		}

		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetIndent("", "  ")
		err := enc.Encode(result)
		require.NoError(t, err)

		var decoded importResult
		err = json.Unmarshal(buf.Bytes(), &decoded)
		require.NoError(t, err)
		assert.Equal(t, 1, decoded.Version)
		assert.Equal(t, 2, decoded.Summary[display.StepOp("import")])
		assert.Equal(t, 1, decoded.Summary[display.StepOp("same")])
		assert.Len(t, decoded.Resources, 1)
		assert.Equal(t, "test", decoded.Resources[0].Name)
		assert.Equal(t, "bucket-id", decoded.Resources[0].ID)
		assert.True(t, decoded.Resources[0].Protected)
	})

	t.Run("with generated code", func(t *testing.T) {
		t.Parallel()

		result := importResult{
			Version: 1,
			GeneratedCode: &generatedCodeResult{
				Language: "typescript",
				Code:     "import * as aws from \"@pulumi/aws\";\n",
			},
		}

		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		err := enc.Encode(result)
		require.NoError(t, err)

		var decoded importResult
		err = json.Unmarshal(buf.Bytes(), &decoded)
		require.NoError(t, err)
		assert.NotNil(t, decoded.GeneratedCode)
		assert.Equal(t, "typescript", decoded.GeneratedCode.Language)
		assert.Contains(t, decoded.GeneratedCode.Code, "@pulumi/aws")
	})

	t.Run("with import file", func(t *testing.T) {
		t.Parallel()

		result := importResult{
			Version: 1,
			ImportFile: &importFileResult{
				Path: "/tmp/import.json",
				Content: &importFile{
					Resources: []importSpec{
						{
							Type: "aws:s3/bucket:Bucket",
							Name: "test",
							ID:   "bucket-id",
						},
					},
				},
			},
		}

		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		err := enc.Encode(result)
		require.NoError(t, err)

		var decoded importResult
		err = json.Unmarshal(buf.Bytes(), &decoded)
		require.NoError(t, err)
		assert.NotNil(t, decoded.ImportFile)
		assert.Equal(t, "/tmp/import.json", decoded.ImportFile.Path)
		assert.NotNil(t, decoded.ImportFile.Content)
		assert.Len(t, decoded.ImportFile.Content.Resources, 1)
	})

	t.Run("preview only", func(t *testing.T) {
		t.Parallel()

		result := importResult{
			Version:     1,
			PreviewOnly: true,
		}

		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		err := enc.Encode(result)
		require.NoError(t, err)

		var decoded importResult
		err = json.Unmarshal(buf.Bytes(), &decoded)
		require.NoError(t, err)
		assert.True(t, decoded.PreviewOnly)
	})

	t.Run("omit empty fields", func(t *testing.T) {
		t.Parallel()

		result := importResult{
			Version: 1,
		}

		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		err := enc.Encode(result)
		require.NoError(t, err)

		// Check that empty fields are omitted
		output := buf.String()
		assert.NotContains(t, output, "summary")
		assert.NotContains(t, output, "resources")
		assert.NotContains(t, output, "generatedCode")
		assert.NotContains(t, output, "importFile")
		assert.NotContains(t, output, "diagnostics")
		assert.NotContains(t, output, "previewOnly")
		assert.NotContains(t, output, "permalink")
	})
}

func TestBuildImportedResources(t *testing.T) {
	t.Parallel()

	imports := []deploy.Import{
		{
			Type: "aws:s3/bucket:Bucket",
			Name: "mybucket",
			ID:   "bucket-123",
		},
		{
			Type:   "aws:s3/bucketObject:BucketObject",
			Name:   "myobject",
			ID:     "object-456",
			Parent: "urn:pulumi:dev::proj::aws:s3/bucket:Bucket::mybucket",
		},
	}

	stackName := tokens.MustParseStackName("dev")
	projectName := tokens.PackageName("proj")

	resources := buildImportedResources(imports, stackName, projectName, true)

	require.Len(t, resources, 2)

	// First resource (no parent)
	assert.Equal(t, "aws:s3/bucket:Bucket", resources[0].Type)
	assert.Equal(t, "mybucket", resources[0].Name)
	assert.Equal(t, "bucket-123", resources[0].ID)
	assert.Equal(t, "import", resources[0].Operation)
	assert.True(t, resources[0].Protected)
	assert.Empty(t, resources[0].Parent)

	// Second resource (with parent)
	assert.Equal(t, "aws:s3/bucketObject:BucketObject", resources[1].Type)
	assert.Equal(t, "myobject", resources[1].Name)
	assert.Equal(t, "object-456", resources[1].ID)
	assert.Equal(t, "urn:pulumi:dev::proj::aws:s3/bucket:Bucket::mybucket", resources[1].Parent)
}

func TestImportDiagnosticsJSON(t *testing.T) {
	t.Parallel()

	result := importResult{
		Version: 1,
		Diagnostics: []importDiagnostic{
			{
				Severity: "error",
				Message:  "import failed: resource not found",
				URN:      "urn:pulumi:dev::proj::aws:s3/bucket:Bucket::test",
			},
			{
				Severity: "warning",
				Message:  "resource may need manual configuration",
			},
		},
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	err := enc.Encode(result)
	require.NoError(t, err)

	var decoded importResult
	err = json.Unmarshal(buf.Bytes(), &decoded)
	require.NoError(t, err)
	require.Len(t, decoded.Diagnostics, 2)
	assert.Equal(t, "error", decoded.Diagnostics[0].Severity)
	assert.Contains(t, decoded.Diagnostics[0].Message, "resource not found")
	assert.Equal(t, "warning", decoded.Diagnostics[1].Severity)
}
