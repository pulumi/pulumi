// Copyright 2025, Pulumi Corporation.
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

// SkillMetadata represents a Pulumi Neo skill query, as returned by the service's registry APIs.
type SkillMetadata struct {
	Name        string  `json:"name"`
	Publisher   string  `json:"publisher"`
	Source      string  `json:"source"`
	DisplayName string  `json:"displayName"`
	Description *string `json:"description,omitempty"`
	// ReadmeURL is just a pre-signed URL, derived from the artifact key.
	ReadmeURL string `json:"readmeURL"`
	// An URL, valid for at least 5 minutes that you can retrieve the full download
	// bundle for your skill.
	//
	// The bundle will be a .tar.gz.
	DownloadURL string `json:"downloadURL"`
	// A link to the hosting repository.
	//
	// Non-VCS backed skills do not have a repo slug as of now.
	RepoSlug   *string           `json:"repoSlug,omitempty"`
	Visibility Visibility        `json:"visibility"`
	UpdatedAt  time.Time         `json:"updatedAt"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type ListSkillsResponse struct {
	Skills            []SkillMetadata `json:"skills"`
	ContinuationToken *string         `json:"continuationToken,omitempty"`
}

// SkillPublishOp contains the information needed to publish a skill to the registry.
type SkillPublishOp struct {
	// Source is the source of the skill. Typically this is 'private' for skills published to the Pulumi Registry.
	Source string
	// Publisher is the organization that is publishing the skill.
	Publisher string
	// Name is the URL-safe name of the skill.
	Name string
	// Version is the semantic version of the skill that should get published.
	Version semver.Version
	// Archive is a reader containing the skill archive (.tar.gz).
	Archive io.Reader
}
