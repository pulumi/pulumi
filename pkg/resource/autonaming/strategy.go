// Copyright 2016-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package autonaming

import (
	"fmt"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/urn"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// autonamingStrategy is a strategy for autonaming a resource.
type autonamingStrategy interface {
	GetOptions(urn urn.URN, randomSeed []byte) (opts *plugin.AutonamingOptions, deleteBeforeCreate bool)
}

// defaultAutonaming is the default autonaming config, which is equivalent to
// no custom autonaming.
type defaultAutonaming struct{}

// defaultAutonamingConfig is the default instance of defaultAutonaming.
var defaultAutonamingConfig = defaultAutonaming{}

func (a *defaultAutonaming) GetOptions(urn.URN, []byte) (opts *plugin.AutonamingOptions, deleteBeforeCreate bool) {
	return nil, false
}

// verbatimAutonaming is an autonaming config that enforces the use of a
// logical resource name as the physical resource name literally, with no transformations.
type verbatimAutonaming struct{}

func (a *verbatimAutonaming) GetOptions(urn urn.URN, _ []byte,
) (opts *plugin.AutonamingOptions, deleteBeforeCreate bool) {
	return &plugin.AutonamingOptions{
		ProposedName: urn.Name(),
		Mode:         plugin.AutonamingModeEnforce,
	}, true
}

// disabledAutonaming is an autonaming config that disables autonaming altogether.
type disabledAutonaming struct{}

func (a *disabledAutonaming) GetOptions(urn.URN, []byte) (opts *plugin.AutonamingOptions, deleteBeforeCreate bool) {
	return &plugin.AutonamingOptions{
		Mode: plugin.AutonamingModeDisabled,
	}, true
}

// patternAutonaming is an autonaming config that uses a pattern to generate a name.
type patternAutonaming struct {
	// Pattern is the pattern to use to generate the name.
	Pattern string
	// Enforce, if true, will enforce the use of the generated name, as opposed to proposing it.
	// A proposed name can still be overridden by the provider, while an enforced name cannot.
	Enforce bool
}

func (a *patternAutonaming) GetOptions(urn urn.URN, randomSeed []byte,
) (opts *plugin.AutonamingOptions, deleteBeforeCreate bool) {
	mode := plugin.AutonamingModePropose
	if a.Enforce {
		mode = plugin.AutonamingModeEnforce
	}
	proposedName, hasRandom := generateName(a.Pattern, urn, randomSeed)
	return &plugin.AutonamingOptions{
		ProposedName:    proposedName,
		Mode:            mode,
		WarnIfNoSupport: a.Enforce,
	}, !hasRandom
}

// providerAutonaming represents the configuration for a provider
type providerAutonaming struct {
	// Default is the default autonaming config for the provider unless overridden by a more specific
	// resource config.
	Default autonamingStrategy

	// Resources maps resource types to their specific configurations
	// Key format: provider:module:type (e.g., "aws:s3/bucket:Bucket")
	Resources map[string]autonamingStrategy
}

// globalAutonaming represents the root configuration object for Pulumi autonaming
type globalAutonaming struct {
	// Default is the default autonaming config for all the providers unless overridden by a more specific
	// provider config.
	Default autonamingStrategy

	// Providers maps provider names to their configurations
	// Key format: provider name (e.g., "aws")
	Providers map[string]providerAutonaming
}

func (o *globalAutonaming) pluginOptionsForResourceType(resourceType tokens.Type) (autonamingStrategy, bool, error) {
	token := string(resourceType)

	// Parse resource type into provider and type
	parts := strings.Split(token, ":")
	if len(parts) != 3 {
		return nil, false, fmt.Errorf("invalid resource type format: %s", resourceType)
	}
	provider := parts[0]

	// Check type-specific config
	if pConfig, ok := o.Providers[provider]; ok {
		if rConfig, ok := pConfig.Resources[token]; ok {
			return rConfig, false, nil
		}
		if pConfig.Default != nil {
			return pConfig.Default, false, nil
		}
	}
	// Fall back to global config
	if o.Default != nil {
		return o.Default, true, nil
	}
	return &defaultAutonamingConfig, true, nil
}

func (o *globalAutonaming) AutonamingForResource(urn urn.URN, randomSeed []byte,
) (*plugin.AutonamingOptions, bool, error) {
	naming, isTopLevelOrDefault, err := o.pluginOptionsForResourceType(urn.Type())
	if err != nil {
		return nil, false, err
	}

	opts, deleteBeforeCreate := naming.GetOptions(urn, randomSeed)
	if opts == nil {
		// If the strategy returns nil, it means the user hasn't overridden the default autonaming for this resource.
		return nil, false, nil
	}

	if !isTopLevelOrDefault {
		// If the strategy comes from a provider- or resource-specific config, it's specific enough that the user
		// definitely intended it to apply to this resource. Therefore, we should always warn if it turns out
		// the provider doesn't actually support autonaming customization.
		opts.WarnIfNoSupport = true
	}
	return opts, deleteBeforeCreate, nil
}
