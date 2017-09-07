// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi-fabric/pkg/tokens"
)

func TestURNRoundTripping(t *testing.T) {
	ns := tokens.QName("namespace")
	alloc := tokens.PackageName("foo/bar/baz")
	typ := tokens.Type("bang:boom/fizzle:MajorResource")
	name := tokens.QName("a-swell-resource")
	urn := NewURN(ns, alloc, typ, name)
	assert.Equal(t, ns, urn.Namespace())
	assert.Equal(t, alloc, urn.Alloc())
	assert.Equal(t, typ, urn.Type())
	assert.Equal(t, name, urn.Name())
}
