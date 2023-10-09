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

package eval

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/pulumi/esc"
	"github.com/pulumi/esc/schema"
	"github.com/pulumi/esc/syntax"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func accept() bool {
	return cmdutil.IsTruthy(os.Getenv("PULUMI_ACCEPT"))
}

type errorProvider struct{}

func (errorProvider) Schema() (*schema.Schema, *schema.Schema) {
	return schema.Record(map[string]schema.Builder{"why": schema.String()}).Schema(), schema.Always()
}

func (errorProvider) Open(ctx context.Context, inputs map[string]esc.Value) (esc.Value, error) {
	return esc.Value{}, errors.New(inputs["why"].Value.(string))
}

type testSchemaProvider struct{}

func (testSchemaProvider) Schema() (*schema.Schema, *schema.Schema) {
	s := schema.Object().
		Defs(map[string]schema.Builder{
			"defRecord": schema.Record(map[string]schema.Builder{
				"baz": schema.String().Const("qux"),
			}),
		}).
		Properties(map[string]schema.Builder{
			"null":    schema.Null(),
			"boolean": schema.Boolean(),
			"false":   schema.Boolean().Const(false),
			"true":    schema.Boolean().Const(true),
			"number":  schema.Number(),
			"pi":      schema.Number().Const("3.14"),
			"string":  schema.String(),
			"hello":   schema.String().Const("hello"),
			"array":   schema.Array().Items(schema.Always()),
			"tuple":   schema.Tuple(schema.String().Const("hello"), schema.String().Const("world")),
			"map":     schema.Object().AdditionalProperties(schema.Always()),
			"record": schema.Record(map[string]schema.Builder{
				"foo": schema.String(),
			}),
			"anyOf": schema.AnyOf(schema.String(), schema.Number()),
			"oneOf": schema.OneOf(schema.String(), schema.Number()),
			"ref":   schema.Ref("#/$defs/defRecord"),

			// Complex cases
			"const-array":  &schema.Schema{Type: "array", Const: []any{"hello", json.Number("42")}},
			"const-object": &schema.Schema{Type: "object", Const: map[string]any{"hello": "world"}},
			"enum":         schema.String().Enum("foo", "bar"),
			"never":        schema.Never(),
			"always":       schema.Always(),
			"double":       schema.Tuple(schema.String(), schema.Number()),
			"triple":       schema.Tuple(schema.String(), schema.Number(), schema.Boolean()),
			"dependentReq": schema.Object().
				Properties(map[string]schema.Builder{
					"foo": schema.String(),
					"bar": schema.Number(),
				}).DependentRequired(map[string][]string{"foo": {"bar"}}),
			"multiple":         schema.Number().MultipleOf(json.Number("2")),
			"minimum":          schema.Number().Minimum(json.Number("1")),
			"exclusiveMinimum": schema.Number().ExclusiveMinimum(json.Number("1")),
			"maximum":          schema.Number().Maximum(json.Number("1")),
			"exclusiveMaximum": schema.Number().ExclusiveMaximum(json.Number("1")),
			"minLength":        schema.String().MinLength(1),
			"maxLength":        schema.String().MaxLength(1),
			"pattern":          schema.String().Pattern(`^foo[0-9]+$`),
			"minItems":         schema.Array().MinItems(3),
			"maxItems":         schema.Array().MaxItems(2),
			"minProperties":    schema.Object().MinProperties(1),
			"maxProperties":    schema.Object().MaxProperties(1),
		}).
		Schema()

	return s, s
}

func (testSchemaProvider) Open(ctx context.Context, inputs map[string]esc.Value) (esc.Value, error) {
	return esc.NewValue(inputs), nil
}

type testProvider struct{}

func (testProvider) Schema() (*schema.Schema, *schema.Schema) {
	return schema.Always(), schema.Always()
}

func (testProvider) Open(ctx context.Context, inputs map[string]esc.Value) (esc.Value, error) {
	return esc.NewValue(inputs), nil
}

type testProviders struct{}

func (testProviders) LoadProvider(ctx context.Context, name string) (esc.Provider, error) {
	switch name {
	case "error":
		return errorProvider{}, nil
	case "schema":
		return testSchemaProvider{}, nil
	case "test":
		return testProvider{}, nil
	}
	return nil, fmt.Errorf("unknown provider %q", name)
}

type testEnvironments struct {
	root string
}

func (e *testEnvironments) LoadEnvironment(ctx context.Context, name string) ([]byte, error) {
	return os.ReadFile(filepath.Join(e.root, name+".yaml"))
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

func normalizeSlice[T any](ts []T, each func(t T) T) []T {
	if len(ts) == 0 {
		return nil
	}
	if each != nil {
		for i, t := range ts {
			ts[i] = each(t)
		}
	}
	return ts
}

func normalizeMap[T any](ts map[string]T, each func(t T) T) map[string]T {
	if len(ts) == 0 {
		return nil
	}
	if each != nil {
		for k, t := range ts {
			ts[k] = each(t)
		}
	}
	return ts
}

func normalizeSchema(s *schema.Schema) *schema.Schema {
	if s == nil {
		return s
	}

	s.Defs = normalizeMap(s.Defs, normalizeSchema)
	s.AnyOf = normalizeSlice(s.AnyOf, normalizeSchema)
	s.OneOf = normalizeSlice(s.OneOf, normalizeSchema)
	s.PrefixItems = normalizeSlice(s.PrefixItems, normalizeSchema)
	normalizeSchema(s.Items)
	normalizeSchema(s.AdditionalProperties)
	s.Properties = normalizeMap(s.Properties, normalizeSchema)

	s.Enum = normalizeSlice(s.Enum, nil)
	s.Required = normalizeSlice(s.Required, nil)
	s.DependentRequired = normalizeMap(s.DependentRequired, func(s []string) []string { return normalizeSlice(s, nil) })

	return s
}

func TestEval(t *testing.T) {
	type expectedData struct {
		LoadDiags   syntax.Diagnostics `json:"loadDiags,omitempty"`
		CheckDiags  syntax.Diagnostics `json:"checkDiags,omitempty"`
		EvalDiags   syntax.Diagnostics `json:"evalDiags,omitempty"`
		Environment *esc.Environment   `json:"environment,omitempty"`
	}

	path := filepath.Join("testdata", "eval")
	entries, err := os.ReadDir(path)
	require.NoError(t, err)
	for _, e := range entries {
		t.Run(e.Name(), func(t *testing.T) {
			basePath := filepath.Join(path, e.Name())
			envPath, expectedPath := filepath.Join(basePath, "env.yaml"), filepath.Join(basePath, "expected.json")

			envBytes, err := os.ReadFile(envPath)
			require.NoError(t, err)

			if accept() {
				env, loadDiags, err := LoadYAMLBytes(e.Name(), envBytes)
				require.NoError(t, err)
				sortEnvironmentDiagnostics(loadDiags)

				_, checkDiags := CheckEnvironment(context.Background(), e.Name(), env, testProviders{}, &testEnvironments{basePath})
				sortEnvironmentDiagnostics(checkDiags)

				actual, evalDiags := EvalEnvironment(context.Background(), e.Name(), env, testProviders{}, &testEnvironments{basePath})
				sortEnvironmentDiagnostics(evalDiags)

				bytes, err := json.MarshalIndent(expectedData{
					LoadDiags:   loadDiags,
					CheckDiags:  checkDiags,
					EvalDiags:   evalDiags,
					Environment: actual,
				}, "", "    ")
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

			env, diags, err := LoadYAMLBytes(e.Name(), envBytes)
			require.NoError(t, err)
			sortEnvironmentDiagnostics(diags)
			require.Equal(t, expected.LoadDiags, diags)

			_, diags = CheckEnvironment(context.Background(), e.Name(), env, testProviders{}, &testEnvironments{basePath})
			sortEnvironmentDiagnostics(diags)
			require.Equal(t, expected.CheckDiags, diags)

			actual, diags := EvalEnvironment(context.Background(), e.Name(), env, testProviders{}, &testEnvironments{basePath})
			sortEnvironmentDiagnostics(diags)
			require.Equal(t, expected.EvalDiags, diags)

			// work around a schema comparison issue due to the 'compiled' field by roundtripping through JSON
			actualBytes, err := json.Marshal(actual)
			require.NoError(t, err)
			dec = json.NewDecoder(bytes.NewReader(actualBytes))
			dec.UseNumber()
			err = dec.Decode(&actual)
			require.NoError(t, err)

			// work around a comparison issue when comparing nil slices/maps against zero-length slices/maps
			if actual != nil {
				actual.Exprs = normalizeMap(actual.Exprs, nil)
				actual.Properties = normalizeMap(actual.Properties, nil)
				normalizeSchema(actual.Schema)
			}

			assert.Equal(t, expected.Environment, actual)
		})
	}
}
