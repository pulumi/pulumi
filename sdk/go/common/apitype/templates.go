// Copyright 2016-2025, Pulumi Corporation.
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

package apitype

// A pulumi template remote where the source URL contains
// a valid Pulumi.yaml file.
type PulumiTemplateRemote struct {
	ProjectTemplate
	Name        string `json:"name"`       // The name of the template
	SourceName  string `json:"sourceName"` // The name of the template source - for display purposes
	TemplateURL string `json:"sourceURL"`  // The unique url that identifies the template to the service
	Runtime     string `json:"runtime"`    // The runtime of the template
}

type ListOrgTemplatesResponse struct {
	Templates map[string][]*PulumiTemplateRemote `json:"templates"`
	// OrgHasTemplates is true len(Templates) > 0
	OrgHasTemplates bool `json:"orgHasTemplates"`
	// HasAccessError indicates that the Pulumi service was not able to access a
	// template source in the org.
	HasAccessError bool `json:"hasAccessError"`
	// HasUpstreamError indicates that there was an unspecified error fetching a
	// template.
	HasUpstreamError bool `json:"hasUpstreamError"`
}

// ProjectTemplate is a Pulumi project template manifest.
type ProjectTemplate struct {
	// DisplayName is an optional user friendly name of the template.
	DisplayName string `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	// Description is an optional description of the template.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	// Quickstart contains optional text to be displayed after template creation.
	Quickstart string `json:"quickstart,omitempty" yaml:"quickstart,omitempty"`
	// Config is an optional template config.
	Config map[string]ProjectTemplateConfigValue `json:"config,omitempty" yaml:"config,omitempty"`
	// Important indicates the template is important.
	//
	// Deprecated: We don't use this field any more.
	Important bool `json:"important,omitempty" yaml:"important,omitempty"`
	// Metadata are key/value pairs used to attach additional metadata to a template.
	Metadata map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// ProjectTemplateConfigValue is a config value included in the project template manifest.
type ProjectTemplateConfigValue struct {
	// Description is an optional description for the config value.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	// Default is an optional default value for the config value.
	Default string `json:"default,omitempty" yaml:"default,omitempty"`
	// Secret may be set to true to indicate that the config value should be encrypted.
	Secret bool `json:"secret,omitempty" yaml:"secret,omitempty"`
}
