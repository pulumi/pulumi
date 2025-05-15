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
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/blang/semver"
)

type StartPackagePublishRequest struct {
	// Version is the semver-compliant version of the package to publish.
	Version string `json:"version"`
}

// StartPackagePublishResponse is the response from initiating a package publish.
// It returns presigned URLs to upload package artifacts.
type StartPackagePublishResponse struct {
	// OperationID is the ID uniquely identifying the publishing operation.
	OperationID string `json:"operationID"`

	// UploadUrls is a collection of URLs for uploading package artifacts.
	UploadURLs PackageUpload `json:"uploadURLs"`

	// RequiredHeaders represents headers that the CLI must set in order
	// for the uploads to succeed.
	RequiredHeaders map[string]string `json:"requiredHeaders,omitempty"`
}

type PackageUpload struct {
	// Schema is the URL for uploading the schema file.
	Schema string `json:"schema"`
	// Index is the URL for uploading the README file.
	Index string `json:"index"`
	// InstallationConfiguration is the URL for uploading the installation docs.
	InstallationConfiguration string `json:"installationConfiguration"`
}

// CompletePackagePublishRequest defines the request body for completing a package
// publish operation after all artifacts have been uploaded.
type CompletePackagePublishRequest struct {
	// OperationID is the ID uniquely identifying the publishing operation.
	OperationID string `json:"operationID"`
}

// PackagePublishOp contains the information needed to publish a package to the registry.
type PackagePublishOp struct {
	// Source is the source of the package. Typically this is 'pulumi' for packages published to the Pulumi Registry.
	// Packages loaded from other registries (e.g. 'opentofu') will point to the origin of the package.
	Source string
	// Publisher is the organization that is publishing the package.
	Publisher string
	// Name is the URL-safe name of the package.
	Name string
	// Version is the semantic version of the package that should get published.
	Version semver.Version
	// Schema is a reader containing the JSON schema of the package.
	Schema io.Reader
	// Readme is a reader containing the markdown content of the package's README.
	Readme io.Reader
	// InstallDocs is a reader containing the markdown content of the package's installation documentation.
	// This is optional, and if omitted, the package will not have installation documentation.
	InstallDocs io.Reader
}

type ListPackagesResponse struct {
	Packages          []PackageMetadata `json:"packages"`
	ContinuationToken *string           `json:"continuationToken,omitempty"`
}

type PackageMetadata struct {
	// The name of the package.
	Name string `json:"name"`
	// The publisher of the package.
	Publisher string `json:"publisher"`
	// The source of the package.
	Source string `json:"source"`
	// The version of the package in semver format.
	Version semver.Version `json:"version"`
	// The title/display name of the package.
	Title string `json:"title,omitempty"`
	// The description of the package.
	Description string `json:"description,omitempty"`
	// The URL of the logo for the package.
	LogoURL string `json:"logoUrl,omitempty"`
	// The URL of the repository the package is hosted in.
	RepoURL string `json:"repoUrl,omitempty"`
	// The category of the package.
	Category string `json:"category,omitempty"`
	// Whether the package is featured.
	IsFeatured bool `json:"isFeatured"`
	// The package types, e.g. "native", "component", "bridged"
	PackageTypes []PackageType `json:"packageTypes,omitempty"`
	// The maturity level of the package, e.g. "ga", "public_preview"
	PackageStatus PackageStatus `json:"packageStatus"`
	// The URL of the readme for the package.
	ReadmeURL string `json:"readmeURL"`
	// The URL of the schema for the package.
	SchemaURL string `json:"schemaURL"`
	// The URL to download the plugin at, as found in the schema.
	PluginDownloadURL string `json:"pluginDownloadURL,omitempty"`
	// The date and time the package version was created.
	CreatedAt time.Time `json:"createdAt"`
	// The visibility of the package.
	Visibility Visibility `json:"visibility"`
}

type PackageType string

const (
	// A package that offers native resources.
	PackageTypeNative PackageType = "native"
	// A package that offers component resources.
	PackageTypeComponent PackageType = "component"
	// A package that is bridged from a different ecosystem (e.g. OpenTofu).
	PackageTypeBridged PackageType = "bridged"
)

type PackageStatus struct{ status string }

var (
	PackageStatusGA            = PackageStatus{"ga"}
	PackageStatusPublicPreview = PackageStatus{"public_preview"}
)

func (ps PackageStatus) String() string {
	if ps.status == "" {
		return "<empty>"
	}
	return ps.status
}

func (ps PackageStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(ps.String())
}

func (ps *PackageStatus) UnmarshalJSON(data []byte) error {
	var status string
	if err := json.Unmarshal(data, &status); err != nil {
		return err
	}

	switch status {
	case PackageStatusGA.status, PackageStatusPublicPreview.status:
		ps.status = status
		return nil
	default:
		return fmt.Errorf("unknown package status: %q", status)
	}
}

type Visibility struct{ status string }

var (
	VisibilityPublic  = Visibility{"public"}
	VisibilityPrivate = Visibility{"private"}
)

func (v Visibility) String() string {
	if v.status == "" {
		return "<empty>"
	}
	return v.status
}

func (v Visibility) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.status)
}

func (v *Visibility) UnmarshalJSON(data []byte) error {
	var status string
	if err := json.Unmarshal(data, &status); err != nil {
		return err
	}
	switch status {
	case VisibilityPublic.status, VisibilityPrivate.status:
		*v = Visibility{status}
	default:
		return fmt.Errorf("unknown visibility: %q", status)
	}
	return nil
}
