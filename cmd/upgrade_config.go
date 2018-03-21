// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"os"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// upgradeConfigurationFiles does an upgrade to move from the old configuration system (where we had config stored) in
// workspace settings and Pulumi.yaml, to the new system where configuration data is stored in Pulumi.<stack-name>.yaml
func upgradeConfigurationFiles() error {
	bes, _ := allBackends()

	skippedUpgrade := false

	for _, b := range bes {
		stacks, err := b.ListStacks()
		if err != nil {
			skippedUpgrade = true
			continue
		}

		for _, stack := range stacks {
			stackName := stack.Name()

			// If the new file exists, we can skip upgrading this stack.
			newFile, err := workspace.DetectProjectStackPath(stackName)
			if err != nil {
				return errors.Wrap(err, "upgrade project")
			}

			_, err = os.Stat(newFile)
			if err != nil && !os.IsNotExist(err) {
				return errors.Wrap(err, "upgrading project")
			} else if err == nil {
				// new file was present, skip upgrading this stack.
				continue
			}

			cfg, salt, err := getOldConfiguration(stackName)
			if err != nil {
				return errors.Wrap(err, "upgrading project")
			}

			ps, err := workspace.DetectProjectStack(stackName)
			if err != nil {
				return errors.Wrap(err, "upgrading project")
			}

			ps.Config = cfg
			ps.EncryptionSalt = salt

			if err := workspace.SaveProjectStack(stackName, ps); err != nil {
				return errors.Wrap(err, "upgrading project")
			}

			if err := removeOldConfiguration(stackName); err != nil {
				return errors.Wrap(err, "upgrading project")
			}
		}
	}

	if !skippedUpgrade {
		return removeOldProjectConfiguration()
	}

	return nil
}

// getOldConfiguration reads the configuration for a given stack from the current workspace.  It applies a hierarchy
// of configuration settings based on stack overrides and workspace-wide global settings.  If any of the workspace
// settings had an impact on the values returned, the second return value will be true. It also returns the encryption
// salt used for the stack.
func getOldConfiguration(stackName tokens.QName) (config.Map, string, error) {
	contract.Require(stackName != "", "stackName")

	// Get the workspace and package and get ready to merge their views of the configuration.
	ws, err := workspace.New()
	if err != nil {
		return nil, "", err
	}
	proj, err := workspace.DetectProject()
	if err != nil {
		return nil, "", err
	}

	// We need to apply workspace and project configuration values in the right order.  Basically, we want to
	// end up taking the most specific settings, where per-stack configuration is more specific than global, and
	// project configuration is more specific than workspace.
	result := make(config.Map)

	// First, apply project-local stack-specific configuration.
	if stack, has := proj.StacksDeprecated[stackName]; has {
		for key, value := range stack.Config {
			result[key] = value
		}
	}

	// Now, apply workspace stack-specific configuration.
	if wsStackConfig, has := ws.Settings().ConfigDeprecated[stackName]; has {
		for key, value := range wsStackConfig {
			if _, has := result[key]; !has {
				result[key] = value
			}
		}
	}

	// Next, take anything from the global settings in our project file.
	for key, value := range proj.ConfigDeprecated {
		if _, has := result[key]; !has {
			result[key] = value
		}
	}

	// Finally, take anything left in the workspace's global configuration.
	if wsGlobalConfig, has := ws.Settings().ConfigDeprecated[""]; has {
		for key, value := range wsGlobalConfig {
			if _, has := result[key]; !has {
				result[key] = value
			}
		}
	}

	// Now, get the encryption key. A stack specific one overrides the global one (global encryption keys were
	// deprecated previously)
	encryptionSalt := proj.EncryptionSaltDeprecated
	if stack, has := proj.StacksDeprecated[stackName]; has && stack.EncryptionSalt != "" {
		encryptionSalt = stack.EncryptionSalt
	}

	return result, encryptionSalt, nil
}

// removeOldConfiguration deletes all configuration information about a stack from both the workspace
// and the project file. It does not touch the newly added Pulumi.<stack-name>.yaml file.
func removeOldConfiguration(stackName tokens.QName) error {
	ws, err := workspace.New()
	if err != nil {
		return err
	}
	proj, err := workspace.DetectProject()
	if err != nil {
		return err
	}

	delete(proj.StacksDeprecated, stackName)
	delete(ws.Settings().ConfigDeprecated, stackName)

	if err := ws.Save(); err != nil {
		return err
	}

	return workspace.SaveProject(proj)
}

// removeOldProjectConfiguration deletes all project level configuration information from both the workspace and the
// project file.
func removeOldProjectConfiguration() error {
	ws, err := workspace.New()
	if err != nil {
		return err
	}
	proj, err := workspace.DetectProject()
	if err != nil {
		return err
	}

	proj.EncryptionSaltDeprecated = ""
	proj.ConfigDeprecated = nil
	ws.Settings().ConfigDeprecated = nil

	if err := ws.Save(); err != nil {
		return err
	}

	return workspace.SaveProject(proj)
}
