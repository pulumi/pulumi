// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package plugin

import (
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/tokens"
)

// ConfigSource is an interface that allows a plugin context to fetch configuration data for a plugin named by
// package.
type ConfigSource interface {
	// GetPackageConfig returns the set of configuration parameters for the indicated package, if any.
	GetPackageConfig(pkg tokens.Package) (map[config.Key]string, error)
}
