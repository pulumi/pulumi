// Copyright 2016-2023, Pulumi Corporation.
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
	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	codegenrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/codegen"
	"github.com/segmentio/encoding/json"
)

type clientSpecSource struct {
	client codegenrpc.LoaderClient
	schema *codegenrpc.GetSchemaRequest
}

func (s *clientSpecSource) GetTypeDefSpec(token string) (ComplexTypeSpec, bool, error) {
	rawSpec, ok := s.spec.Types[token]
	if !ok {
		return ComplexTypeSpec{}, false, nil
	}

	var spec ComplexTypeSpec
	if err := parseJSONPropertyValue(rawSpec, &spec); err != nil {
		return ComplexTypeSpec{}, false, err
	}
	return spec, true, nil
}

func (s *clientSpecSource) GetFunctionSpec(token string) (FunctionSpec, bool, error) {
	rawSpec, ok := s.spec.Functions[token]
	if !ok {
		return FunctionSpec{}, false, nil
	}

	var spec FunctionSpec
	if err := parseJSONPropertyValue(rawSpec, &spec); err != nil {
		return FunctionSpec{}, false, err
	}
	return spec, true, nil
}

func (s *clientSpecSource) GetResourceSpec(token string) (ResourceSpec, bool, error) {
	var rawSpec json.RawMessage
	if token == "pulumi:providers:"+s.spec.Name {
		rawSpec = s.spec.Provider
	} else {
		raw, ok := s.spec.Resources[token]
		if !ok {
			return ResourceSpec{}, false, nil
		}
		rawSpec = raw
	}

	var spec ResourceSpec
	if err := parseJSONPropertyValue(rawSpec, &spec); err != nil {
		return ResourceSpec{}, false, err
	}
	return spec, true, nil
}

// ImportClientSpec converts a serializable PartialPackageSpec into a PartialPackage. Unlike a typical Package, a
// PartialPackage loads and binds its members on-demand rather than at import time. This is useful when the entire
// contents of a package are not needed (e.g. for referenced packages).
func ImportClientSpec(client codegenrpc.LoaderClient, schema *codegenrpc.GetSchemaRequest, spec *codegenrpc.PackageSpec, loader Loader) (*PartialPackage, error) {
	marshalVersion := func(v *string) string {
		if v == nil {
			return ""
		}
		return *v
	}

	marshalDependencies := func(deps []*codegenrpc.PackageDescriptor) ([]PackageDescriptor, error) {
		return slice.MapError(deps, func(dep *codegenrpc.PackageDescriptor) (PackageDescriptor, error) {
			if dep == nil {
				return PackageDescriptor{}, nil
			}
			var v *semver.Version
			if dep.Version != "" {
				version, err := semver.Parse(dep.Version)
				if err != nil {
					return PackageDescriptor{}, err
				}
				v = &version
			}

			var parameterization *ParameterizationDescriptor
			if dep.Parameterization != nil {
				paramVersion, err := semver.Parse(dep.Parameterization.Version)
				if err != nil {
					return PackageDescriptor{}, err
				}
				dep.Parameterization.Version = paramVersion.String()
			}

			return PackageDescriptor{
				Name:             dep.Package,
				Version:          v,
				DownloadURL:      dep.DownloadUrl,
				Parameterization: parameterization,
			}, nil
		})
	}

	marshalMeta := func(meta *codegenrpc.MetaSpec) *MetadataSpec {
		if meta == nil {
			return nil
		}
		return &MetadataSpec{
			ModuleFormat: meta.ModuleFormat,
			SupportPack:  meta.SupportPack,
		}
	}

	marshalParameterization := func(param *codegenrpc.ParameterizationSpec) *ParameterizationSpec {
		if param == nil {
			return nil
		}
		return &ParameterizationSpec{
			BaseProvider: BaseProviderSpec{
				Name:    param.BaseProviderName,
				Version: param.BaseProviderVersion,
			},
			Parameter: param.Parameter,
		}
	}

	dependencies, err := marshalDependencies(spec.Dependencies)
	if err != nil {
		return nil, err
	}

	partialSpec := &PartialPackageSpec{
		PackageInfoSpec: PackageInfoSpec{
			Name:                spec.Name,
			DisplayName:         spec.DisplayName,
			Version:             marshalVersion(spec.Version),
			Description:         spec.Description,
			Keywords:            spec.Keywords,
			Homepage:            spec.Homepage,
			License:             spec.License,
			Attribution:         spec.Attribution,
			Repository:          spec.Repository,
			LogoURL:             spec.LogoUrl,
			PluginDownloadURL:   spec.PluginDownloadUrl,
			Publisher:           spec.Publisher,
			Namespace:           spec.Namespace,
			Dependencies:        dependencies,
			Meta:                marshalMeta(spec.Meta),
			AllowedPackageNames: spec.AllowedPackageNames,
			Parameterization:    marshalParameterization(spec.Parameterization),
		},
	}

	pkg := &PartialPackage{
		spec:      partialSpec,
		languages: nil,
	}
	types, diags, err := newBinder(partialSpec.PackageInfoSpec, &clientSpecSource{client, schema}, loader, pkg)
	if err != nil {
		return nil, err
	}
	if diags.HasErrors() {
		return nil, diags
	}
	pkg.types = types
	return pkg, nil
}
