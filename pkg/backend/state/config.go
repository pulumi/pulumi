// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package state

import (
	"sort"

	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// Configuration reads the configuration for a given stack from the current workspace.  It applies a hierarchy of
// configuration settings based on stack overrides and workspace-wide global settings.  If any of the workspace
// settings had an impact on the values returned, the second return value will be true.
func Configuration(d diag.Sink, stackName tokens.QName) (config.Map, error) {
	contract.Require(stackName != "", "stackName")

	// Get the workspace and package and get ready to merge their views of the configuration.
	ws, err := workspace.New()
	if err != nil {
		return nil, err
	}
	proj, err := workspace.DetectProject()
	if err != nil {
		return nil, err
	}

	// We need to apply workspace and project configuration values in the right order.  Basically, we want to
	// end up taking the most specific settings, where per-stack configuration is more specific than global, and
	// project configuration is more specific than workspace.
	result := make(config.Map)
	var workspaceConfigKeys []string

	// First, apply project-local stack-specific configuration.
	if stack, has := proj.Stacks[stackName]; has {
		for key, value := range stack.Config {
			result[key] = value
		}
	}

	// Now, apply workspace stack-specific configuration.
	if wsStackConfig, has := ws.Settings().Config[stackName]; has {
		for key, value := range wsStackConfig {
			if _, has := result[key]; !has {
				result[key] = value
				workspaceConfigKeys = append(workspaceConfigKeys, string(key))
			}
		}
	}

	// Next, take anything from the global settings in our project file.
	for key, value := range proj.Config {
		if _, has := result[key]; !has {
			result[key] = value
		}
	}

	// Finally, take anything left in the workspace's global configuration.
	if wsGlobalConfig, has := ws.Settings().Config[""]; has {
		for key, value := range wsGlobalConfig {
			if _, has := result[key]; !has {
				result[key] = value
				workspaceConfigKeys = append(workspaceConfigKeys, string(key))
			}
		}
	}

	// If there are any configuration settings from the workspace being used, issue a warning.  This can be a subtle
	// source of discrepancy when deploying stacks to the cloud, and can be tough to track down.
	if len(workspaceConfigKeys) > 0 {
		sort.Strings(workspaceConfigKeys)
		d.Warningf(
			diag.Message("configuration variables were taken from your local workspace; proceed with caution: %v"),
			workspaceConfigKeys)
	}

	return result, nil
}
