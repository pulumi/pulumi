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

package operations

import (
	"github.com/pulumi/pulumi/pkg/v3/display"
)

// importResult is the JSON output structure for pulumi import.
// Existing fields should not be removed or renamed to maintain backwards compatibility.
type importResult struct {
	// Version is the schema version for this output format.
	Version int `json:"version"`
	// Summary contains the count of resources by operation type (e.g., "import": 2).
	Summary display.ResourceChanges `json:"summary,omitempty"`
	// Resources contains details about each imported resource.
	Resources []importedResource `json:"resources,omitempty"`
	// GeneratedCode contains the generated code for the imported resources.
	GeneratedCode *generatedCodeResult `json:"generatedCode,omitempty"`
	// ImportFile contains information about any generated import file.
	ImportFile *importFileResult `json:"importFile,omitempty"`
	// Diagnostics contains any warnings or errors encountered during import.
	Diagnostics []importDiagnostic `json:"diagnostics,omitempty"`
	// PreviewOnly is true if this was a preview-only operation.
	PreviewOnly bool `json:"previewOnly,omitempty"`
	// Permalink is the URL to view this update in the Pulumi Cloud.
	Permalink string `json:"permalink,omitempty"`
}

// importedResource describes a single resource that was imported.
type importedResource struct {
	// URN is the Pulumi URN for the imported resource.
	URN string `json:"urn"`
	// Type is the Pulumi type token for the resource.
	Type string `json:"type"`
	// Name is the Pulumi name (logical name) for the resource.
	Name string `json:"name"`
	// ID is the cloud provider's ID for the resource.
	ID string `json:"id,omitempty"`
	// Operation describes what happened to this resource (e.g., "import", "same").
	Operation string `json:"operation"`
	// Protected indicates if the resource is protected from deletion.
	Protected bool `json:"protected,omitempty"`
	// Parent is the URN of the parent resource, if any.
	Parent string `json:"parent,omitempty"`
	// Provider is the URN of the provider used for this resource.
	Provider string `json:"provider,omitempty"`
}

// generatedCodeResult contains information about generated code.
type generatedCodeResult struct {
	// Language is the programming language of the generated code.
	Language string `json:"language"`
	// Code is the generated source code. Empty if written to a file.
	Code string `json:"code,omitempty"`
	// FilePath is the path where the code was written, if --out was specified.
	FilePath string `json:"filePath,omitempty"`
	// Warning contains any warning message about code generation.
	Warning string `json:"warning,omitempty"`
}

// importFileResult contains information about a generated import file.
type importFileResult struct {
	// Path is the file path where the import file was written.
	Path string `json:"path"`
	// Content is the parsed content of the import file, if available.
	Content *importFile `json:"content,omitempty"`
}

// importDiagnostic represents a warning or error that occurred during import.
type importDiagnostic struct {
	// Severity is "warning" or "error".
	Severity string `json:"severity"`
	// Message is the diagnostic message.
	Message string `json:"message"`
	// URN is the resource URN this diagnostic relates to, if applicable.
	URN string `json:"urn,omitempty"`
}
