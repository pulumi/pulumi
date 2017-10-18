// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/pack"
	"github.com/pulumi/pulumi/pkg/tokens"

	"github.com/stretchr/testify/assert"
)

func TestPrettyKeyForPackage(t *testing.T) {
	pkg := pack.Package{Name: tokens.PackageName("test-package"), Runtime: "nodejs"}

	assert.Equal(t, "foo", prettyKeyForPackage("test-package:config:foo", pkg))
	assert.Equal(t, "other-package:config:bar", prettyKeyForPackage("other-package:config:bar", pkg))
}
