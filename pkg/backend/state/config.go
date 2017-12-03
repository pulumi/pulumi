// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package state

import (
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// Configuration reads the configuration for a given stack from the current workspace.  It applies a hierarchy of
// configuration settings based on stack overrides and workspace-wide global settings.
func Configuration(stackName tokens.QName) (config.Map, error) {
	contract.Require(stackName != "", "stackName")

	config := make(config.Map)

	// First apply global configs from the package.
	pkg, err := workspace.GetPackage()
	if err != nil {
		return nil, err
	}
	for key, value := range pkg.Config {
		config[key] = value
	}

	// Next, apply configs from this particular stack, if any.
	if stackInfo, has := pkg.Stacks[stackName]; has {
		for key, value := range stackInfo.Config {
			config[key] = value
		}
	}

	// Now, if there are global settings in the workspace, apply those.
	ws, err := workspace.New()
	if err != nil {
		return nil, err
	}
	if localAllStackConfig, has := ws.Settings().Config[""]; has {
		for key, value := range localAllStackConfig {
			config[key] = value
		}
	}

	// And finally, if there are global workspace settings for this stack, apply those.
	if localStackConfig, has := ws.Settings().Config[stackName]; has {
		for key, value := range localStackConfig {
			config[key] = value
		}
	}

	return config, nil
}
