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
