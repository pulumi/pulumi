package gen

import (
	"bytes"
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
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
	packages, err := generatePackageContextMap(tool, pkg.Reference(), goPkgInfo, nil)
	if err != nil {
		return nil, err
	}

	pkgMods := slice.Prealloc[string](len(packages))
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
			pkg.genHeader(buffer, []string{"context", "reflect"}, importsAndAliases, false /* isUtil */)

			if err := pkg.genResource(buffer, r, goPkgInfo.GenerateResourceContainerTypes, false); err != nil {
				return nil, fmt.Errorf("generating resource %s: %w", mod, err)
			}
		}

		if len(pkg.types) > 0 {
			for _, t := range pkg.types {
				if err := pkg.genType(buffer, t, false); err != nil {
					return nil, err
				}
			}
			pkg.genTypeRegistrations(buffer, pkg.types, false /* usingGenericTypes */)
		}

		buffers[mod] = buffer
	}

	return buffers, nil
}
