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

package docs

import (
	"context"
	"fmt"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

type staticSchemaLoader struct {
	schema           *schema.Package
	schemaReferences map[string]*schema.Package
}

var _ schema.ReferenceLoader = (*staticSchemaLoader)(nil)

func NewStaticSchemaLoader(loadedSchema *schema.Package) schema.ReferenceLoader {
	// TODO: resolve schema references from input schema
	references := map[string]*schema.Package{}

	return &staticSchemaLoader{
		schema:           loadedSchema,
		schemaReferences: references,
	}
}

func (loader *staticSchemaLoader) LoadPackage(pkg string, version *semver.Version) (*schema.Package, error) {
	if loader.schema.Name == pkg {
		return loader.schema, nil
	}

	if ref, ok := loader.schemaReferences[pkg]; ok {
		return ref, nil
	}

	return nil, fmt.Errorf("package %s not found", pkg)
}

func (loader *staticSchemaLoader) LoadPackageV2(
	ctx context.Context, descriptor *schema.PackageDescriptor,
) (*schema.Package, error) {
	return loader.LoadPackage(descriptor.Name, descriptor.Version)
}

func (loader *staticSchemaLoader) LoadPackageReference(
	pkg string,
	version *semver.Version,
) (schema.PackageReference, error) {
	loadedPackage, err := loader.LoadPackage(pkg, version)
	if err != nil {
		return nil, err
	}

	return loadedPackage.Reference(), nil
}

func (loader *staticSchemaLoader) LoadPackageReferenceV2(
	ctx context.Context, descriptor *schema.PackageDescriptor,
) (schema.PackageReference, error) {
	return loader.LoadPackageReference(descriptor.Name, descriptor.Version)
}
