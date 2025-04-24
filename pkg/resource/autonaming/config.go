// Copyright 2024, Pulumi Corporation.
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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
)

func resolveNamingConfig(c *namingConfigJSON, eval *stackPatternEval) (Autonamer, bool, error) {
	hasMode := c.Mode != nil
	hasPattern := c.Pattern != nil
	hasEnforce := c.Enforce != nil

	if hasMode && (hasPattern || hasEnforce) {
		return nil, false, errors.New("cannot specify both mode and pattern/enforce")
	}

	if hasPattern {
		pattern, err := eval.resolveStackExpressions(*c.Pattern)
		if err != nil {
			return nil, false, err
		}

		return &patternAutonaming{
			Pattern: pattern,
			Enforce: hasEnforce && *c.Enforce,
		}, true, nil
	} else if hasEnforce {
		return nil, false, errors.New("pattern must be specified when enforce is true")
	}

	if !hasMode {
		// Return false as the second return value to indicate that no config was specified explicitly.
		return &defaultAutonamingConfig, false, nil
	}

	switch *c.Mode {
	case "default":
		return &defaultAutonamingConfig, true, nil
	case "verbatim":
		return &verbatimAutonaming{}, true, nil
	case "disabled":
		return &disabledAutonaming{}, true, nil
	default:
		return nil, false, fmt.Errorf("invalid naming mode: %s", *c.Mode)
	}
}

func parseConfigSection(v config.Value) (*autonamingSectionJSON, error) {
	vInterface, err := v.ToObject()
	if err != nil {
		return nil, fmt.Errorf("invalid autonaming config: %w", err)
	}

	b, err := json.Marshal(vInterface)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal autonaming config: %w", err)
	}

	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.DisallowUnknownFields()
	var autonamingConfig *autonamingSectionJSON
	if err := decoder.Decode(&autonamingConfig); err != nil {
		return nil, fmt.Errorf("invalid autonaming config structure: %w", err)
	}

	return autonamingConfig, nil
}

func ParseAutonamingConfig(s StackContext, cfg config.Map, decrypter config.Decrypter) (Autonamer, error) {
	// Get the autonaming config from the stack configuration, return nil if it's not set.
	v, ok, err := cfg.Get(config.MustParseKey("pulumi:autonaming"), false)
	if err != nil {
		return nil, fmt.Errorf("failed to get autonaming config: %w", err)
	}
	if !ok {
		return nil, nil
	}

	// Parse the autonaming config into a strongly-typed struct.
	autonamingConfig, err := parseConfigSection(v)
	if err != nil {
		return nil, fmt.Errorf("failed to parse autonaming config: %w", err)
	}

	// Helper evaluator for resolving stack-level expressions in the autonaming patterns.
	eval := newStackPatternEval(s, cfg, decrypter)

	// Resolve the root naming config.
	rootNaming, _, err := resolveNamingConfig(&autonamingConfig.namingConfigJSON, eval)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve root naming config: %w", err)
	}

	// Initialize the global autonaming config.
	result := &globalAutonaming{
		Default:   rootNaming,
		Providers: make(map[string]providerAutonaming),
	}

	// Resolve the provider-level naming configs.
	for providerName, providerCfg := range autonamingConfig.Providers {
		providerCfg := providerCfg
		naming, has, err := resolveNamingConfig(&providerCfg.namingConfigJSON, eval)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve naming config for provider %q: %w", providerName, err)
		}
		if !has {
			// If no provider config was specified explicitly, use the global default.
			naming = rootNaming
		}

		provider := providerAutonaming{
			Default:   naming,
			Resources: make(map[string]Autonamer),
		}

		// Resolve the resource-level naming configs.
		for resourceName, resourceCfg := range providerCfg.Resources {
			resourceCfg := resourceCfg
			resourceNaming, has, err := resolveNamingConfig(&resourceCfg, eval)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve naming config for resource %q: %w", resourceName, err)
			}
			if !has {
				// If no config was specified explicitly, that's probably a mistake by the user
				// why would they define the resource entry otherwise.
				return nil, fmt.Errorf("mode or pattern must be specified for resource %q", resourceName)
			}

			provider.Resources[resourceName] = resourceNaming
		}

		result.Providers[providerName] = provider
	}

	return result, nil
}
