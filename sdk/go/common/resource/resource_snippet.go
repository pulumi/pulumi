// Copyright 2026, Pulumi Corporation.
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

package resource

import "github.com/blang/semver"

// ParameterizationDescriptor is the serializable description of a dependency's parameterization.
type ParameterizationDescriptor struct {
	// Name is the name of the package.
	Name string `json:"name" yaml:"name"`
	// Version is the version of the package.
	Version semver.Version `json:"version" yaml:"version"`
	// Value is the parameter value of the package.
	Value []byte `json:"value" yaml:"value"`
}

// PackageDescriptor is a descriptor for a package, this is similar to a plugin spec but also contains parameterization
// info.
type PackageDescriptor struct {
	// Name is the simple name of the plugin.
	Name string `json:"name" yaml:"name"`
	// Version is the optional version of the plugin.
	Version *semver.Version `json:"version,omitempty" yaml:"version,omitempty"`
	// DownloadURL is the optional URL to use when downloading the provider plugin binary.
	DownloadURL string `json:"downloadURL,omitempty" yaml:"downloadURL,omitempty"`
	// Parameterization is the optional parameterization of the package.
	Parameterization *ParameterizationDescriptor `json:"parameterization,omitempty" yaml:"parameterization,omitempty"`
}

// Snippet represents a snippet of PCL that should be associated with a stack. The engine reruns these in deployments.
//
//nolint:lll
type Snippet struct {
	// The logical name of the resource this snippet is for.
	Name string `json:"name" yaml:"name"`
	// The type of the resource this snippet is for.
	Type string `json:"type" yaml:"type"`
	// The PCL code for an expression for the body of this resource.
	Code string `json:"code" yaml:"code"`
	// The package descriptor for the resource
	Descriptor PackageDescriptor `json:"descriptor" yaml:"descriptor"`
	// References declares the external resources that this snippet's Code may refer to by HCL identifier. The map key
	// is the identifier used inside Code; the value identifies the target resource by its URN. The target need not be
	// present in the snapshot, but it must be registered during an update before the snippet can run.
	References map[string]string `json:"references,omitempty" yaml:"references,omitempty"`
}
