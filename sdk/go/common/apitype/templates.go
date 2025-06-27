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

import (
	"io"
	"time"

	"github.com/blang/semver"
)

// TemplateMetadata represents a template query, as returned by the service's registry
// APIs.
type TemplateMetadata struct {
	Name        string  `json:"name"`
	Publisher   string  `json:"publisher"`
	Source      string  `json:"source"`
	DisplayName string  `json:"displayName"`
	Description *string `json:"description,omitempty"`
	// The language that the template is in.
	Language string `json:"language"`
	// ReadmeURL is just a pre-signed URL, derived from the artifact key.
	ReadmeURL string `json:"readmeURL"`
	// An URL, valid for at least 5 minutes that you can retrieve the full download
	// bundle for your template.
	//
	// The bundle will be a .tar.gz.
	DownloadURL string `json:"downloadURL"`
	// A link to the hosting repository.
	//
	// Non-VCS backed templates do not have a repo slug as of now.
	RepoSlug   *string           `json:"repoSlug,omitempty"`
	Visibility Visibility        `json:"visibility"`
	UpdatedAt  time.Time         `json:"updatedAt"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type ListTemplatesResponse struct {
	Templates         []TemplateMetadata `json:"templates"`
	ContinuationToken *string            `json:"continuationToken,omitempty"`
}

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

// TemplatePublishOp contains the information needed to publish a template to the registry.
type TemplatePublishOp struct {
	// Source is the source of the template. Typically this is 'private' for templates published to the Pulumi Registry.
	Source string
	// Publisher is the organization that is publishing the template.
	Publisher string
	// Name is the URL-safe name of the template.
	Name string
	// Version is the semantic version of the template that should get published.
	Version semver.Version
	// Archive is a reader containing the template archive (.tar.gz).
	Archive io.Reader
}
