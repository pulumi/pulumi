// Copyright 2026, Pulumi Corporation.
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

package resources

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

func TestBuildRowsAndFilter(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)
	earlier := now.Add(-24 * time.Hour)

	states := []*resource.State{
		{
			URN:      "urn:pulumi:prod::myproj::aws:s3/bucket:Bucket::logs",
			Type:     "aws:s3/bucket:Bucket",
			Custom:   true,
			Modified: &now,
		},
		{
			URN:      "urn:pulumi:prod::myproj::aws:ec2/instance:Instance::web",
			Type:     "aws:ec2/instance:Instance",
			Custom:   true,
			Protect:  true,
			Modified: &earlier,
		},
		{
			URN:  "urn:pulumi:prod::myproj::pulumi:providers:aws::default",
			Type: "pulumi:providers:aws",
		},
	}

	rows := buildRows(states)
	require.Len(t, rows, 3)
	assert.Equal(t, "logs", rows[0].Name)
	assert.Equal(t, "myproj", rows[0].Project)
	assert.Equal(t, "prod", rows[0].Stack)
	assert.Equal(t, "active", rows[0].Status)
	assert.True(t, rows[1].Protected)

	// Glob filter on type.
	filtered, err := filterRows(rows, listArgs{typeGlob: "aws:s3/*"})
	require.NoError(t, err)
	require.Len(t, filtered, 1)
	assert.Equal(t, "logs", filtered[0].Name)

	// Substring fallback when no glob metacharacters.
	filtered, err = filterRows(rows, listArgs{typeGlob: "ec2"})
	require.NoError(t, err)
	require.Len(t, filtered, 1)
	assert.Equal(t, "web", filtered[0].Name)

	// Status filter.
	filtered, err = filterRows(rows, listArgs{status: "active"})
	require.NoError(t, err)
	assert.Len(t, filtered, 3)
}

func TestSortAndPaginate(t *testing.T) {
	t.Parallel()
	rows := []resourceRow{
		{Name: "c", Type: "t1"},
		{Name: "a", Type: "t2"},
		{Name: "b", Type: "t1"},
	}
	require.NoError(t, sortRows(rows, "name", "asc", ""))
	assert.Equal(t, []string{"a", "b", "c"},
		[]string{rows[0].Name, rows[1].Name, rows[2].Name})

	require.NoError(t, sortRows(rows, "name", "desc", ""))
	assert.Equal(t, []string{"c", "b", "a"},
		[]string{rows[0].Name, rows[1].Name, rows[2].Name})

	// group-by type then sort by name ascending.
	rows = []resourceRow{
		{Name: "b", Type: "t2"},
		{Name: "a", Type: "t1"},
		{Name: "c", Type: "t2"},
	}
	require.NoError(t, sortRows(rows, "name", "asc", "type"))
	assert.Equal(t, "t1", rows[0].Type)
	assert.Equal(t, "b", rows[1].Name)
	assert.Equal(t, "c", rows[2].Name)

	// paginate.
	paged := paginateRows(rows, 1, 1)
	require.Len(t, paged, 1)
	assert.Equal(t, rows[1], paged[0])

	// sort with unknown field errors.
	require.Error(t, sortRows(rows, "bogus", "asc", ""))
}

func TestRenderJSON(t *testing.T) {
	t.Parallel()
	output := listOutput{
		Resources:  []resourceRow{{Name: "logs", Type: "aws:s3/bucket:Bucket", Status: "active"}},
		TotalCount: 1,
		Page:       listPage{Offset: 0, Limit: 0},
		Query:      listQuery{Sort: listSort{Field: "name", Order: "asc"}},
	}
	var buf bytes.Buffer
	require.NoError(t, renderOutput(&buf, "json", defaultColumns, output, true))

	var decoded listOutput
	require.NoError(t, json.Unmarshal(buf.Bytes(), &decoded))
	assert.Equal(t, 1, decoded.TotalCount)
	assert.Equal(t, "logs", decoded.Resources[0].Name)
}

func TestRenderJSONL(t *testing.T) {
	t.Parallel()
	output := listOutput{
		Resources: []resourceRow{
			{Name: "a", Type: "t1", Status: "active"},
			{Name: "b", Type: "t1", Status: "active"},
		},
	}
	var buf bytes.Buffer
	require.NoError(t, renderOutput(&buf, "jsonl", defaultColumns, output, true))

	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	require.Len(t, lines, 2)
	var first resourceRow
	require.NoError(t, json.Unmarshal(lines[0], &first))
	assert.Equal(t, "a", first.Name)
}

func TestResolveOutputFormat(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	format, err := resolveOutputFormat("", &buf)
	require.NoError(t, err)
	assert.Equal(t, "json", format)

	format, err = resolveOutputFormat("yaml", &buf)
	require.NoError(t, err)
	assert.Equal(t, "yaml", format)

	format, err = resolveOutputFormat("yml", &buf)
	require.NoError(t, err)
	assert.Equal(t, "yaml", format)

	_, err = resolveOutputFormat("bogus", &buf)
	require.Error(t, err)
}

func TestResolveColumns(t *testing.T) {
	t.Parallel()
	cols, err := resolveColumns("")
	require.NoError(t, err)
	assert.Equal(t, defaultColumns, cols)

	cols, err = resolveColumns("name,type,urn")
	require.NoError(t, err)
	assert.Equal(t, []string{"name", "type", "urn"}, cols)

	_, err = resolveColumns("name,bogus")
	require.Error(t, err)
}
