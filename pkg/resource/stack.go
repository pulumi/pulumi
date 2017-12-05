// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package resource

import (
	"github.com/pulumi/pulumi/pkg/tokens"
)

// RootStackType is the type name that will be used for the root component in the Pulumi resource tree.
const RootStackType tokens.Type = "pulumi:pulumi:Stack"
