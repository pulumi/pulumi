// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package workspace

import (
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/tokens"
)

// Settings defines workspace settings shared amongst many related projects.
// nolint: lll
type Settings struct {
	Stack            tokens.QName                `json:"stack,omitempty" yaml:"env,omitempty"`     // an optional default stack to use.
	ConfigDeprecated map[tokens.QName]config.Map `json:"config,omitempty" yaml:"config,omitempty"` // optional workspace local configuration (overrides values in a project)
}
