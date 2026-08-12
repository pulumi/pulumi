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

// autonamingSectionJSON represents the root configuration object for Pulumi autonaming
// Example of a configuration that it encodes (config will convert it to JSON before autonaming receives it):
//
// pulumi:autonaming:
//
//	mode: default
//	providers:
//	  aws:
//	    pattern: ${name}_${hex(4)}
//	  azure-native:
//	    mode: verbatim
//	    resources:
//	      "azure-native:storage:Account": ${name}${string(6)}
type autonamingSectionJSON struct {
	namingConfigJSON

	// Providers maps provider names to their configurations
	// Key format: provider name (e.g., "aws")
	Providers map[string]providerConfigJSON `json:"providers,omitempty"`
}

// providerConfigJSON represents the configuration for a provider
type providerConfigJSON struct {
	namingConfigJSON

	// Resources maps resource types to their specific configurations
	// Key format: provider:module:type (e.g., "aws:s3/bucket:Bucket")
	Resources map[string]namingConfigJSON `json:"resources,omitempty"`
}

// namingConfigJSON represents the base configuration for resource naming.
// The same set of options can be specified globally, per-provider, or per-resource.
type namingConfigJSON struct {
	// Mode specifies the autonaming mode: default (standard Pulumi behavior),
	// verbatim (use logical names), or disabled (require explicit names)
	Mode *string `json:"mode,omitempty"`

	// Pattern is a template string for custom name generation.
	// Example: "${stack}-${name}-${hex(6)}"
	Pattern *string `json:"pattern,omitempty"`

	// Enforce prevents providers from modifying the specified naming pattern when true
	Enforce *bool `json:"enforce,omitempty"`
}
