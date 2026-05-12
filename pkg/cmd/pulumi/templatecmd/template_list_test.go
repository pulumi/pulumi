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

package templatecmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"iter"
	"testing"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// templatesFromIterable yields the given templates as a registry list iterator.
func templatesFromIterable(templates []apitype.TemplateMetadata) iter.Seq2[apitype.TemplateMetadata, error] {
	return func(yield func(apitype.TemplateMetadata, error) bool) {
		for _, tmpl := range templates {
			if !yield(tmpl, nil) {
				return
			}
		}
	}
}

// mockListRegistry stubs registry.Registry's ListTemplates and panics on every
// other method. The test only needs ListTemplates wired up.
type mockListRegistry struct {
	registry.Mock
}

func newMockListRegistry(
	t *testing.T, captured *registry.ListTemplatesOptions, templates []apitype.TemplateMetadata, err error,
) *mockListRegistry {
	t.Helper()
	r := &mockListRegistry{}
	r.ListTemplatesF = func(
		_ context.Context, opts registry.ListTemplatesOptions,
	) iter.Seq2[apitype.TemplateMetadata, error] {
		if captured != nil {
			*captured = opts
		}
		if err != nil {
			return func(yield func(apitype.TemplateMetadata, error) bool) {
				yield(apitype.TemplateMetadata{}, err)
			}
		}
		return templatesFromIterable(templates)
	}
	return r
}

func registryFactory(r registry.Registry) func(ctx context.Context) registry.Registry {
	return func(_ context.Context) registry.Registry { return r }
}

func sampleTemplates() []apitype.TemplateMetadata {
	desc := "An example template"
	return []apitype.TemplateMetadata{
		{
			Name:        "aws-quickstart",
			Publisher:   "pulumi",
			Source:      "private",
			DisplayName: "AWS Quickstart",
			Description: &desc,
			Language:    "typescript",
			Visibility:  apitype.VisibilityPublic,
			UpdatedAt:   time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
		},
		{
			Name:       "gcp-starter",
			Publisher:  "acme",
			Source:     "private",
			Language:   "python",
			Visibility: apitype.VisibilityPrivate,
			UpdatedAt:  time.Date(2026, 3, 4, 8, 0, 0, 0, time.UTC),
		},
	}
}

func TestTemplateListCmd_DefaultOutput_WithResults(t *testing.T) {
	t.Parallel()

	reg := newMockListRegistry(t, nil, sampleTemplates(), nil)
	c := &templateListCmd{registryFactory: registryFactory(reg)}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, templateListArgs{})
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "aws-quickstart")
	assert.Contains(t, output, "gcp-starter")
	assert.Contains(t, output, "Name")
	assert.Contains(t, output, "Publisher")
	assert.Contains(t, output, "2026-01-15")
}

func TestTemplateListCmd_DefaultOutput_NoResults(t *testing.T) {
	t.Parallel()

	reg := newMockListRegistry(t, nil, nil, nil)
	c := &templateListCmd{registryFactory: registryFactory(reg)}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, templateListArgs{})
	require.NoError(t, err)

	assert.Equal(t, "No templates found.\n", out.String())
}

func TestTemplateListCmd_JSONOutput(t *testing.T) {
	t.Parallel()

	reg := newMockListRegistry(t, nil, sampleTemplates(), nil)
	c := &templateListCmd{registryFactory: registryFactory(reg)}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, templateListArgs{output: "json"})
	require.NoError(t, err)

	var got struct {
		Templates []apitype.TemplateMetadata `json:"templates"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	assert.Equal(t, sampleTemplates(), got.Templates)
}

func TestTemplateListCmd_JSONOutput_NoResults(t *testing.T) {
	t.Parallel()

	reg := newMockListRegistry(t, nil, nil, nil)
	c := &templateListCmd{registryFactory: registryFactory(reg)}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, templateListArgs{output: "json"})
	require.NoError(t, err)

	// Empty list, not null — keeps the contract stable for scripts.
	assert.JSONEq(t, `{"templates":[]}`, out.String())
}

func TestTemplateListCmd_FiltersPassedThrough(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args templateListArgs
		want registry.ListTemplatesOptions
	}{
		{
			name: "name only",
			args: templateListArgs{name: "foo"},
			want: registry.ListTemplatesOptions{Name: "foo"},
		},
		{
			name: "org only",
			args: templateListArgs{org: "acme"},
			want: registry.ListTemplatesOptions{Org: "acme"},
		},
		{
			name: "search only",
			args: templateListArgs{search: "serverless"},
			want: registry.ListTemplatesOptions{Search: "serverless"},
		},
		{
			name: "all three",
			args: templateListArgs{name: "foo", org: "acme", search: "bar"},
			want: registry.ListTemplatesOptions{Name: "foo", Org: "acme", Search: "bar"},
		},
		{
			name: "no filters",
			args: templateListArgs{},
			want: registry.ListTemplatesOptions{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var captured registry.ListTemplatesOptions
			reg := newMockListRegistry(t, &captured, nil, nil)
			c := &templateListCmd{registryFactory: registryFactory(reg)}

			var out bytes.Buffer
			err := c.Run(t.Context(), &out, tt.args)
			require.NoError(t, err)
			assert.Equal(t, tt.want, captured)
		})
	}
}

func TestTemplateListCmd_InvalidOutput(t *testing.T) {
	t.Parallel()

	reg := newMockListRegistry(t, nil, sampleTemplates(), nil)
	c := &templateListCmd{registryFactory: registryFactory(reg)}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, templateListArgs{output: "yaml"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid --output value "yaml"`)
}

func TestTemplateListCmd_RegistryError(t *testing.T) {
	t.Parallel()

	reg := newMockListRegistry(t, nil, nil, errors.New("connection refused"))
	c := &templateListCmd{registryFactory: registryFactory(reg)}

	var out bytes.Buffer
	err := c.Run(t.Context(), &out, templateListArgs{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing templates")
	assert.Contains(t, err.Error(), "connection refused")
}

func TestNewTemplateListCmd_FlagsAndAliases(t *testing.T) {
	t.Parallel()

	var captured registry.ListTemplatesOptions
	reg := newMockListRegistry(t, &captured, sampleTemplates(), nil)

	cmd := NewTemplateListCmd(registryFactory(reg))
	assert.Equal(t, "list", cmd.Name())
	assert.Contains(t, cmd.Aliases, "ls")

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"--name", "foo",
		"--org", "acme",
		"--search", "bar",
		"--output", "json",
	})
	require.NoError(t, cmd.ExecuteContext(t.Context()))

	assert.Equal(t, registry.ListTemplatesOptions{Name: "foo", Org: "acme", Search: "bar"}, captured)

	var got struct {
		Templates []apitype.TemplateMetadata `json:"templates"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	require.Len(t, got.Templates, 2)
}

func TestNewTemplateListCmd_NilFactoryUsesDefault(t *testing.T) {
	t.Parallel()

	// Constructing with nil installs the default factory; we only verify the
	// command is well-formed without actually invoking it (the default factory
	// would touch the workspace and Pulumi Cloud).
	cmd := NewTemplateListCmd(nil)
	require.NotNil(t, cmd)
	assert.Equal(t, "list", cmd.Name())
}
