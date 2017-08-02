// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package core

import (
	"github.com/pulumi/pulumi-fabric/pkg/tokens"
)

// Args are a set of command line arguments supplied during evaluation.
type Args map[tokens.Name]interface{}
