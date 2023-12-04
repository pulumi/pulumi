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
	"testing"

	"github.com/stretchr/testify/assert"

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
	}
	for _, str := range goodUrns {
		urn := URN(str)
		assert.True(t, urn.IsValid(), "IsValid expected to be true: %v", urn)
	}
}
