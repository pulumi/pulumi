// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package workspace

import (
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/tokens"
)

// Settings defines workspace settings shared amongst many related projects.
// nolint: lll
type Settings struct {
	Namespace string                                                `json:"namespace,omitempty" yaml:"namespace,omitempty"` // an optional namespace.
	Stack     tokens.QName                                          `json:"env,omitempty" yaml:"env,omitempty"`             // an optional default stack to use.
	Config    map[tokens.QName]map[tokens.ModuleMember]config.Value `json:"config,omitempty" yaml:"config,omitempty"`       // optional workspace local configuration (overrides values in a project)
}
