// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/tokens"
)

func TestURNRoundTripping(t *testing.T) {
	ns := tokens.QName("namespace")
	alloc := tokens.PackageName("foo/bar/baz")
	parentType := tokens.Type("")
	typ := tokens.Type("bang:boom/fizzle:MajorResource")
	name := tokens.QName("a-swell-resource")
	urn := NewURN(ns, alloc, parentType, typ, name)
	assert.Equal(t, ns, urn.Namespace())
	assert.Equal(t, alloc, urn.Alloc())
	assert.Equal(t, tokens.Type("!!bang:boom/fizzle:MajorResource"), urn.QualifiedType())
	assert.Equal(t, typ, urn.Type())
	assert.Equal(t, name, urn.Name())
}

func TestURNRoundTripping2(t *testing.T) {
	ns := tokens.QName("namespace")
	alloc := tokens.PackageName("foo/bar/baz")
	parentType := tokens.Type("parent!!type")
	typ := tokens.Type("bang:boom/fizzle:MajorResource")
	name := tokens.QName("a-swell-resource")
	urn := NewURN(ns, alloc, parentType, typ, name)
	assert.Equal(t, ns, urn.Namespace())
	assert.Equal(t, alloc, urn.Alloc())
	assert.Equal(t, tokens.Type("parent!!type!!bang:boom/fizzle:MajorResource"), urn.QualifiedType())
	assert.Equal(t, typ, urn.Type())
	assert.Equal(t, name, urn.Name())
}
