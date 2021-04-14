package gen

import (
	"bytes"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

// CRDTypes returns a map from each module name to a buffer containing the
// code for its generated types.
func CRDTypes(tool string, pkg *schema.Package) (map[string]*bytes.Buffer, error) {
	if err := pkg.ImportLanguages(map[string]schema.Language{"go": Importer}); err != nil {
		return map[string]*bytes.Buffer{}, err
	}

	var goPkgInfo GoPackageInfo
	if goInfo, ok := pkg.Language["go"].(GoPackageInfo); ok {
		goPkgInfo = goInfo
	}
	packages := generatePackageContextMap(tool, pkg, goPkgInfo)

	var pkgMods []string
	for mod := range packages {
		pkgMods = append(pkgMods, mod)
	}

	buffers := map[string]*bytes.Buffer{}

	for _, mod := range pkgMods {
		pkg := packages[mod]
		buffer := &bytes.Buffer{}

		for _, r := range pkg.resources {
			importsAndAliases := map[string]string{}
			pkg.getImports(r, importsAndAliases)
			pkg.genHeader(buffer, []string{"context", "reflect"}, importsAndAliases)

			if err := pkg.genResource(buffer, r, goPkgInfo.GenerateResourceContainerTypes); err != nil {
				return nil, errors.Wrapf(err, "generating resource %s", mod)
			}
		}

		if len(pkg.types) > 0 {
			for _, t := range pkg.types {
				pkg.genType(buffer, t)
			}
			pkg.genTypeRegistrations(buffer, pkg.types)
		}

		buffers[mod] = buffer
	}

	return buffers, nil
}
