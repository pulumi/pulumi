package docs

import (
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
