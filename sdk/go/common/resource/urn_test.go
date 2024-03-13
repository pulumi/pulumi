// Copyright 2016-2018, Pulumi Corporation.
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

package resource

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

func TestURNRoundTripping(t *testing.T) {
	t.Parallel()

	stack := tokens.QName("stck")
	proj := tokens.PackageName("foo/bar/baz")
	parentType := tokens.Type("")
	typ := tokens.Type("bang:boom/fizzle:MajorResource")
	name := "a-swell-resource"
	urn := NewURN(stack, proj, parentType, typ, name)
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
	urn := NewURN(stack, proj, parentType, typ, name)
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
	urn := NewURN(stack, proj, parentType, typ, name)
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
		urn := URN(str)
		assert.True(t, urn.IsValid(), "IsValid expected to be true: %v", urn)
	}
}

func TestParseURN(t *testing.T) {
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
			urn, err := ParseURN(str)
			assert.NoErrorf(t, err, "Expecting %v to parse as a good urn", str)
			assert.Equal(t, str, string(urn), "A parsed URN should be the same as the string that it was parsed from")
		}
	})

	t.Run("Negative Tests", func(t *testing.T) {
		t.Parallel()

		t.Run("Empty String", func(t *testing.T) {
			t.Parallel()

			urn, err := ParseURN("")
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
				urn, err := ParseURN(str)
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
			urn, err := ParseOptionalURN(str)
			assert.NoErrorf(t, err, "Expecting '%v' to parse as a good urn", str)
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
			urn, err := ParseOptionalURN(str)
			assert.ErrorContainsf(t, err, "invalid URN", "Expecting %v to parse as an invalid urn")
			assert.Empty(t, urn)
		}
	})
}

func TestQuote(t *testing.T) {
	t.Parallel()

	urn, err := ParseURN("urn:pulumi:test::test::pulumi:pulumi:Stack::test-test")
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

	urn := NewURN(stack, proj, parentType, typ, name)
	renamed := urn.Rename("a-better-resource")

	assert.NotEqual(t, urn, renamed)
	assert.Equal(t,
		NewURN(stack, proj, parentType, typ, "a-better-resource"),
		renamed)
}

func TestName(t *testing.T) {
	t.Parallel()

	stack := tokens.QName("stack")
	proj := tokens.PackageName("project")
	parentType := tokens.Type("parent$type")
	typ := tokens.Type("bang:boom/fizzle:MajorResource")

	names := []string{
		"test",
		"a-longer-name",
		"a name with spaces",
		"a-name-with::a-name-separator",
		"a-name-with::many::name::separators",
	}

	for _, name := range names {
		urn := NewURN(stack, proj, parentType, typ, name)
		require.NotEmpty(t, urn)

		assert.Equal(t, name, urn.Name())
	}
}
