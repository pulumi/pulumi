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

package schemainfo

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
)

func TestSummarize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "empty", input: "", expected: ""},
		{name: "single line", input: "A short description.", expected: "A short description."},
		{
			name:     "collapses newlines within the first paragraph",
			input:    "The first\nparagraph continues.\n\nA second paragraph is dropped.",
			expected: "The first paragraph continues.",
		},
		{
			name:     "resolves language-choice spans",
			input:    "Conflicts with <span pulumi-lang-nodejs=\"`imageUri`\">`imageUri`</span>.",
			expected: "Conflicts with `imageUri`.",
		},
		{
			name:     "skips leading blank lines",
			input:    "\n\nThe actual content.\n\nDropped.",
			expected: "The actual content.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, Summarize(tt.input))
		})
	}
}

func TestTypeString(t *testing.T) {
	t.Parallel()

	input := func(t schema.Type) schema.Type { return &schema.InputType{ElementType: t} }
	optional := func(t schema.Type) schema.Type { return &schema.OptionalType{ElementType: t} }

	tests := []struct {
		name     string
		typ      schema.Type
		expected string
	}{
		{name: "primitive", typ: schema.StringType, expected: "string"},
		{name: "outer optional and input are elided", typ: optional(input(schema.BoolType)), expected: "boolean"},
		{
			name:     "inner input wrappers are preserved",
			typ:      input(&schema.ArrayType{ElementType: input(schema.StringType)}),
			expected: "Array<Input<string>>",
		},
		{name: "map", typ: &schema.MapType{ElementType: schema.StringType}, expected: "Map<string>"},
		{
			name:     "object reference keeps its full token",
			typ:      &schema.ObjectType{Token: "aws:lambda/function:FunctionConfig"},
			expected: "aws:lambda/function:FunctionConfig",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, TypeString(tt.typ))
		})
	}
}

func TestBoundProperties(t *testing.T) {
	t.Parallel()

	props := []*schema.Property{
		{Name: "name", Type: schema.StringType, Comment: "The name."},
		{Name: "size", Type: &schema.OptionalType{ElementType: schema.IntType}},
	}
	got := BoundProperties(props)
	assert.Equal(t, []Property{
		{Name: "name", Type: "string", Required: true, Description: "The name."},
		{Name: "size", Type: "integer", Required: false},
	}, got)
}

func TestWriteProperties(t *testing.T) {
	t.Parallel()

	t.Run("writes in the given order, marks required, and adds the inputs footnote", func(t *testing.T) {
		t.Parallel()
		var b strings.Builder
		WriteProperties(&b, colors.Never, "Inputs", []Property{
			{Name: "size", Type: "integer"},
			{Name: "name", Type: "string", Required: true, Description: "The name.\n\nMore detail."},
			{Name: "tags", Type: "[]string"},
		}, Inputs)
		assert.Equal(t, "Inputs:\n"+
			" - size (integer)\n"+
			" - name (string*): The name.\n"+
			" - tags ([]string)\n"+
			"Inputs marked with '*' are required\n", b.String())
	})

	t.Run("uses the outputs footnote wording", func(t *testing.T) {
		t.Parallel()
		var b strings.Builder
		WriteProperties(&b, colors.Never, "Outputs", []Property{
			{Name: "id", Type: "string", Required: true, Description: "The id."},
		}, Outputs)
		assert.Equal(t, "Outputs:\n"+
			" - id (string*): The id.\n"+
			"Outputs marked with '*' are always present\n", b.String())
	})

	t.Run("uses the list-inputs footnote wording", func(t *testing.T) {
		t.Parallel()
		var b strings.Builder
		WriteProperties(&b, colors.Never, "List Inputs", []Property{
			{Name: "prefix", Type: "string", Required: true, Description: "The prefix."},
		}, ListInputs)
		assert.Equal(t, "List Inputs:\n"+
			" - prefix (string*): The prefix.\n"+
			"List inputs marked with '*' are required\n", b.String())
	})

	t.Run("writes the title with no footnote for an empty or unmarked section", func(t *testing.T) {
		t.Parallel()
		var b strings.Builder
		WriteProperties(&b, colors.Never, "Outputs", nil, Outputs)
		assert.Equal(t, "Outputs:\n", b.String())
	})

	t.Run("styles names bold and types underlined when colorization is enabled", func(t *testing.T) {
		t.Parallel()
		var b strings.Builder
		WriteProperties(&b, colors.Always, "Inputs", []Property{{Name: "name", Type: "string"}}, Inputs)
		assert.Contains(t, b.String(), Bold(colors.Always, "name"))
		assert.Contains(t, b.String(), Underline(colors.Always, "string"))
	})
}

func TestCleanComment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty",
			input:    "",
			expected: "",
		},
		{
			name:     "no markup",
			input:    "Function entry point in your code.",
			expected: "Function entry point in your code.",
		},
		{
			name: "language-choice span renders the canonical camelCase choice",
			input: "Required if <span pulumi-lang-nodejs=\"`packageType`\" " +
				"pulumi-lang-python=\"`package_type`\">`packageType`</span> is `Zip`.",
			expected: "Required if `packageType` is `Zip`.",
		},
		{
			name: "multiple spans on one line are handled independently",
			input: "Conflicts with <span pulumi-lang-go=\"`imageUri`\">`imageUri`</span> " +
				"and <span pulumi-lang-go=\"`s3Bucket`\">`s3Bucket`</span>.",
			expected: "Conflicts with `imageUri` and `s3Bucket`.",
		},
		{
			name:     "span nested in backticks",
			input:    "Valid values are `[<span pulumi-lang-python=\"x86_64\">\"x8664\"</span>]` and `[\"arm64\"]`.",
			expected: "Valid values are `[\"x8664\"]` and `[\"arm64\"]`.",
		},
		{
			name:     "literal angle-bracket placeholders are left untouched",
			input:    "Use the form arn:aws:s3:::<bucket>/<key>.",
			expected: "Use the form arn:aws:s3:::<bucket>/<key>.",
		},
		{
			name: "a paired env var collapses to the uppercase name",
			input: "Can also be set using the `HTTP_PROXY` or " +
				"<span pulumi-lang-nodejs=\"`httpProxy`\" " +
				"pulumi-lang-python=\"`http_proxy`\">`httpProxy`</span> environment variables.",
			expected: "Can also be set using the `HTTP_PROXY` environment variable.",
		},
		{
			name:     "a lone env var is left untouched",
			input:    "Can also be configured using the `AWS_RETRY_MODE` environment variable.",
			expected: "Can also be configured using the `AWS_RETRY_MODE` environment variable.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, CleanComment(tt.input))
		})
	}
}
