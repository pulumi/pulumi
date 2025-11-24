// Copyright 2025-2025, Pulumi Corporation.
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

package schema

import (
	"context"

	codegenrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/codegen"
)

// LoaderPackageReference is a PackageReference that loads its schema on demand using a Loader supporting
// GetPackageInfo and related partial methods.
type LoaderPackageReference struct {
	loader PartialLoader

	request *codegenrpc.GetSchemaRequest
	info    *codegenrpc.PackageInfo
}

var _ PackageReference = (*LoaderPackageReference)(nil)

func NewLoaderPackageReference(ctx context.Context, loader PartialLoader, request *codegenrpc.GetSchemaRequest) (*LoaderPackageReference, error) {
	info, err := loader.GetPackageInfo(ctx, request)
	if err != nil {
		return nil, err
	}

	return &LoaderPackageReference{
		loader:  loader,
		request: request,
		info:    info,
	}, nil
}

func (l *LoaderPackageReference) Name() string {
	return l.info.Name
}

func (l *LoaderPackageReference) Version() string {
	v := l.info.Version
	if v == nil {
		return ""
	}
	return *v
}

func (l *LoaderPackageReference) Description() string {
	return l.info.Description
}

func (l *LoaderPackageReference) Publisher() string {
	return l.info.Publisher
}

func (l *LoaderPackageReference) Namespace() string {
	if l.info.Namespace == nil {
		return "pulumi"
	}
	return *l.info.Namespace
}

func (l *LoaderPackageReference) Repository() string {
	return l.info.Repository
}

func (l *LoaderPackageReference) SupportPack() bool {
	if l.info.Meta == nil {
		return false
	}
	return l.info.Meta.SupportPack
}

func (l *LoaderPackageReference) Definition() (*Package, error) {
	var metaSpec *MetadataSpec
	if l.info.Meta != nil {
		metaSpec = &MetadataSpec{
			ModuleFormat: l.info.Meta.ModuleFormat,
			SupportPack:  l.info.Meta.SupportPack,
		}
	}

	var language map[string]RawMessage
	if len(l.info.Languages) > 0 {
		language = make(map[string]RawMessage, len(l.info.Languages))
		for lang, langData := range l.info.Languages {
			language[lang] = RawMessage(langData)
		}
	}

	var configSpec ConfigSpec
	var typesSpec map[string]ComplexTypeSpec
	var provider ResourceSpec
	var resources map[string]ResourceSpec

	/* Provider ResourceSpec `json:"provider,omitempty" yaml:"provider"`
	   // Resources is a map from type token to ResourceSpec that describes the set of resources defined by this package.
	   Resources map[string]ResourceSpec `json:"resources,omitempty" yaml:"resources,omitempty"`
	   // Functions is a map from token to FunctionSpec that describes the set of functions defined by this package.
	   Functions map[string]FunctionSpec `json:"functions,omitempty" yaml:"functions,omitempty"`
	   // Dependencies is a list of dependencies of this packaeg
	   Dependencies []PackageDescriptor `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`

	   // Parameterization is the optional parameterization for this package.
	   Parameterization *ParameterizationSpec `json:"parameterization,omitempty" yaml:"parameterization,omitempty"`
	*/

	spec := PackageSpec{
		Name:                l.info.Name,
		DisplayName:         l.info.DisplayName,
		Version:             l.Version(),
		Description:         l.info.Description,
		Keywords:            l.info.Keywords,
		Homepage:            l.info.Homepage,
		License:             l.info.License,
		Attribution:         l.info.Attribution,
		Repository:          l.info.Repository,
		LogoURL:             l.info.LogoUrl,
		PluginDownloadURL:   l.info.PluginDownloadUrl,
		Publisher:           l.info.Publisher,
		Namespace:           l.Namespace(),
		Meta:                metaSpec,
		AllowedPackageNames: l.info.AllowedPackageNames,
		Language:            language,
		Config:              configSpec,
		Types:               typesSpec,
	}

	pkg, diagnostics, err := BindSpec(spec, l.loader, ValidationOptions{})
	if err != nil {
		return nil, err
	}
	if diagnostics.HasErrors() {
		return nil, diagnostics
	}

	return pkg, nil
}
