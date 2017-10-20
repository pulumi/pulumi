// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package workspace

import (
	"github.com/pulumi/pulumi/pkg/tokens"
)

// Settings defines workspace settings shared amongst many related projects.
type Settings struct {
	Namespace string       `json:"namespace,omitempty" yaml:"namespace,omitempty"` // an optional namespace.
	Stack     tokens.QName `json:"env,omitempty" yaml:"env,omitempty"`             // an optional default stack to use.
}
