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

package urn_test

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/urn"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

func TestURNRoundTripping(t *testing.T) {
	t.Parallel()

	stack := tokens.QName("stck")
	proj := tokens.PackageName("foo/bar/baz")
	parentType := tokens.Type("")
	typ := tokens.Type("bang:boom/fizzle:MajorResource")
	name := "a-swell-resource"
	urn := urn.New(stack, proj, parentType, typ, name)
	assert.Equal(t, stack, urn.Stack())
	assert.Equal(t, proj, urn.Project())
	assert.Equal(t, typ, urn.QualifiedType())
	assert.Equal(t, typ, urn.Type())
	assert.Equal(t, name, urn.Name())
}

func TestURNRoundTripping2(t *testing.T) {
	t.Parallel()

	stack := tokens.QName("stck")
	proj := tokens.PackageName("foo/bar/baz")
	parentType := tokens.Type("parent$type")
	typ := tokens.Type("bang:boom/fizzle:MajorResource")
	name := "a-swell-resource"
	urn := urn.New(stack, proj, parentType, typ, name)
	assert.Equal(t, stack, urn.Stack())
	assert.Equal(t, proj, urn.Project())
	assert.Equal(t, tokens.Type("parent$type$bang:boom/fizzle:MajorResource"), urn.QualifiedType())
	assert.Equal(t, typ, urn.Type())
	assert.Equal(t, name, urn.Name())
}

func TestURNRoundTripping3(t *testing.T) {
	t.Parallel()

	stack := tokens.QName("stck")
	proj := tokens.PackageName("foo/bar/baz")
	parentType := tokens.Type("parent$type")
	typ := tokens.Type("bang:boom/fizzle:MajorResource")
	name := "a-swell-resource::with_awkward$names"
	urn := urn.New(stack, proj, parentType, typ, name)
	assert.Equal(t, stack, urn.Stack())
	assert.Equal(t, proj, urn.Project())
	assert.Equal(t, tokens.Type("parent$type$bang:boom/fizzle:MajorResource"), urn.QualifiedType())
	assert.Equal(t, typ, urn.Type())
	assert.Equal(t, name, urn.Name())
}

func TestIsValid(t *testing.T) {
	t.Parallel()

	goodUrns := []string{
		"urn:pulumi:test::test::pulumi:pulumi:Stack::test-test",
		"urn:pulumi:stack-name::project-name::my:customtype$aws:s3/bucket:Bucket::bob",
		"urn:pulumi:stack::project::type::",
		"urn:pulumi:stack::project::type::some really ::^&\n*():: crazy name",
		"urn:pulumi:stack::project with whitespace::type::some name",
	}
	for _, str := range goodUrns {
		urn := urn.URN(str)
		assert.True(t, urn.IsValid(), "IsValid expected to be true: %v", urn)
	}
}

func TestComponentAccess(t *testing.T) {
	t.Parallel()

	type ComponentTestCase struct {
		urn      string
		expected string
	}

	t.Run("Stack component", func(t *testing.T) {
		t.Parallel()

		cases := []ComponentTestCase{
			{urn: "urn:pulumi:stack::test::pulumi:pulumi:Stack::test-test", expected: "stack"},
			{urn: "urn:pulumi:stack::::::", expected: "stack"},
			{urn: "urn:pulumi:::test::pulumi:pulumi:Stack::test-test", expected: ""},
			{urn: "urn:pulumi:::::::", expected: ""},
		}

		for _, test := range cases {
			urn, err := urn.Parse(test.urn)
			require.NoError(t, err)
			require.Equal(t, test.urn, string(urn))

			assert.Equalf(t, tokens.QName(test.expected), urn.Stack(),
				"Expecting stack to be '%v' from urn '%v'", test.expected, test.urn)
		}
	})

	t.Run("Project component", func(t *testing.T) {
		t.Parallel()

		cases := []ComponentTestCase{
			{urn: "urn:pulumi:stack::proj::pulumi:pulumi:Stack::test-test", expected: "proj"},
			{urn: "urn:pulumi:::proj::::", expected: "proj"},
			{urn: "urn:pulumi:stack::::pulumi:pulumi:Stack::test-test", expected: ""},
			{urn: "urn:pulumi:::::::", expected: ""},
		}

		for _, test := range cases {
			urn, err := urn.Parse(test.urn)
			require.NoError(t, err)
			require.Equal(t, test.urn, string(urn))

			assert.Equalf(t, tokens.PackageName(test.expected), urn.Project(),
				"Expecting project to be '%v' from urn '%v'", test.expected, test.urn)
		}
	})

	t.Run("QualifiedType component", func(t *testing.T) {
		t.Parallel()

		cases := []ComponentTestCase{
			{urn: "urn:pulumi:stack::proj::qualified$type::test-test", expected: "qualified$type"},
			{urn: "urn:pulumi:::::qualified$type::", expected: "qualified$type"},
			{urn: "urn:pulumi:stack::proj::::test-test", expected: ""},
			{urn: "urn:pulumi:::::::", expected: ""},
		}

		for _, test := range cases {
			urn, err := urn.Parse(test.urn)
			require.NoError(t, err)
			require.Equal(t, test.urn, string(urn))

			assert.Equalf(t, tokens.Type(test.expected), urn.QualifiedType(),
				"Expecting qualified type to be '%v' from urn '%v'", test.expected, test.urn)
		}
	})

	t.Run("Type component", func(t *testing.T) {
		t.Parallel()

		cases := []ComponentTestCase{
			{urn: "urn:pulumi:stack::proj::very$qualified$type::test-test", expected: "type"},
			{urn: "urn:pulumi:::::very$qualified$type::", expected: "type"},
			{urn: "urn:pulumi:stack::proj::qualified$type::test-test", expected: "type"},
			{urn: "urn:pulumi:::::qualified$type::", expected: "type"},
			{urn: "urn:pulumi:stack::proj::qualified-type::test-test", expected: "qualified-type"},
			{urn: "urn:pulumi:::::qualified-type::", expected: "qualified-type"},
			{urn: "urn:pulumi:stack::proj::::test-test", expected: ""},
			{urn: "urn:pulumi:::::::", expected: ""},
		}

		for _, test := range cases {
			urn, err := urn.Parse(test.urn)
			require.NoError(t, err)
			require.Equal(t, test.urn, string(urn))

			assert.Equalf(t, tokens.Type(test.expected), urn.Type(),
				"Expecting type to be '%v' from urn '%v'", test.expected, test.urn)
		}
	})

	t.Run("Name component", func(t *testing.T) {
		t.Parallel()

		cases := []ComponentTestCase{
			{urn: "urn:pulumi:stack::proj::qualified$type::name", expected: "name"},
			{urn: "urn:pulumi:::::::name", expected: "name"},
			{urn: "urn:pulumi:stack::proj::qualified$type::", expected: ""},
			{urn: "urn:pulumi:::::::", expected: ""},
			{
				urn:      "urn:pulumi::stack::proj::type::a-longer-name",
				expected: "a-longer-name",
			},
			{
				urn:      "urn:pulumi::stack::proj::type::a name with spaces",
				expected: "a name with spaces",
			},
			{
				urn:      "urn:pulumi::stack::proj::type::a-name-with::a-name-separator",
				expected: "a-name-with::a-name-separator",
			},
			{
				urn:      "urn:pulumi::stack::proj::type::a-name-with::many::name::separators",
				expected: "a-name-with::many::name::separators",
			},
		}

		for _, test := range cases {
			urn, err := urn.Parse(test.urn)
			require.NoError(t, err)
			require.Equal(t, test.urn, string(urn))

			assert.Equalf(t, test.expected, urn.Name(),
				"Expecting name to be '%v' from urn '%v'", test.expected, test.urn)
		}
	})
}

func TestURNParse(t *testing.T) {
	t.Parallel()

	t.Run("Positive Tests", func(t *testing.T) {
		t.Parallel()

		goodUrns := []string{
			"urn:pulumi:test::test::pulumi:pulumi:Stack::test-test",
			"urn:pulumi:stack-name::project-name::my:customtype$aws:s3/bucket:Bucket::bob",
			"urn:pulumi:stack::project::type::",
			"urn:pulumi:stack::project::type::some really ::^&\n*():: crazy name",
			"urn:pulumi:stack::project with whitespace::type::some name",
		}
		for _, str := range goodUrns {
			urn, err := urn.Parse(str)
			require.NoErrorf(t, err, "Expecting %v to parse as a good urn", str)
			assert.Equal(t, str, string(urn), "A parsed URN should be the same as the string that it was parsed from")
		}
	})

	t.Run("Negative Tests", func(t *testing.T) {
		t.Parallel()

		t.Run("Empty String", func(t *testing.T) {
			t.Parallel()

			urn, err := urn.Parse("")
			assert.ErrorContains(t, err, "missing required URN")
			assert.Empty(t, urn)
		})

		t.Run("Invalid URNs", func(t *testing.T) {
			t.Parallel()

			invalidUrns := []string{
				"URN:PULUMI:TEST::TEST::PULUMI:PULUMI:STACK::TEST-TEST",
				"urn:not-pulumi:stack-name::project-name::my:customtype$aws:s3/bucket:Bucket::bob",
				"The quick brown fox",
				"urn:pulumi:stack::too-few-elements",
			}
			for _, str := range invalidUrns {
				urn, err := urn.Parse(str)
				assert.ErrorContainsf(t, err, "invalid URN", "Expecting %v to parse as an invalid urn")
				assert.Empty(t, urn)
			}
		})
	})
}

func TestParseOptionalURN(t *testing.T) {
	t.Parallel()

	t.Run("Positive Tests", func(t *testing.T) {
		t.Parallel()

		goodUrns := []string{
			"urn:pulumi:test::test::pulumi:pulumi:Stack::test-test",
			"urn:pulumi:stack-name::project-name::my:customtype$aws:s3/bucket:Bucket::bob",
			"urn:pulumi:stack::project::type::",
			"urn:pulumi:stack::project::type::some really ::^&\n*():: crazy name",
			"urn:pulumi:stack::project with whitespace::type::some name",
			"",
		}
		for _, str := range goodUrns {
			urn, err := urn.ParseOptional(str)
			require.NoErrorf(t, err, "Expecting '%v' to parse as a good urn", str)
			assert.Equal(t, str, string(urn))
		}
	})

	t.Run("Invalid URNs", func(t *testing.T) {
		t.Parallel()

		invalidUrns := []string{
			"URN:PULUMI:TEST::TEST::PULUMI:PULUMI:STACK::TEST-TEST",
			"urn:not-pulumi:stack-name::project-name::my:customtype$aws:s3/bucket:Bucket::bob",
			"The quick brown fox",
			"urn:pulumi:stack::too-few-elements",
		}
		for _, str := range invalidUrns {
			urn, err := urn.ParseOptional(str)
			assert.ErrorContainsf(t, err, "invalid URN", "Expecting %v to parse as an invalid urn")
			assert.Empty(t, urn)
		}
	})
}

func TestQuote(t *testing.T) {
	t.Parallel()

	urn, err := urn.Parse("urn:pulumi:test::test::pulumi:pulumi:Stack::test-test")
	require.NoError(t, err)
	require.NotEmpty(t, urn)

	expected := "'urn:pulumi:test::test::pulumi:pulumi:Stack::test-test'"
	if runtime.GOOS == "windows" {
		expected = "\"urn:pulumi:test::test::pulumi:pulumi:Stack::test-test\""
	}

	assert.Equal(t, expected, urn.Quote())
}

func TestRename(t *testing.T) {
	t.Parallel()

	stack := tokens.QName("stack")
	proj := tokens.PackageName("foo/bar/baz")
	parentType := tokens.Type("parent$type")
	typ := tokens.Type("bang:boom/fizzle:MajorResource")
	name := "a-swell-resource"

	oldURN := urn.New(stack, proj, parentType, typ, name)
	renamed := oldURN.Rename("a-better-resource")

	assert.NotEqual(t, oldURN, renamed)
	assert.Equal(t,
		urn.New(stack, proj, parentType, typ, "a-better-resource"),
		renamed)
}

func TestRenameStack(t *testing.T) {
	t.Parallel()

	stack := tokens.QName("stack")
	proj := tokens.PackageName("foo/bar/baz")
	parentType := tokens.Type("parent$type")
	typ := tokens.Type("bang:boom/fizzle:MajorResource")
	name := "a-swell-resource"

	oldURN := urn.New(stack, proj, parentType, typ, name)
	renamed := oldURN.RenameStack(tokens.MustParseStackName("a-better-stack"))

	assert.NotEqual(t, oldURN, renamed)
	assert.Equal(t,
		urn.New("a-better-stack", proj, parentType, typ, name),
		renamed)
}

func TestRenameProject(t *testing.T) {
	t.Parallel()

	stack := tokens.QName("stack")
	proj := tokens.PackageName("foo/bar/baz")
	parentType := tokens.Type("parent$type")
	typ := tokens.Type("bang:boom/fizzle:MajorResource")
	name := "a-swell-resource"

	oldURN := urn.New(stack, proj, parentType, typ, name)
	renamed := oldURN.RenameProject(tokens.PackageName("a-better-project"))

	assert.NotEqual(t, oldURN, renamed)
	assert.Equal(t,
		urn.New(stack, "a-better-project", parentType, typ, name),
		renamed)
}
