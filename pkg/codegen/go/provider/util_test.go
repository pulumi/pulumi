package provider

import "golang.org/x/tools/go/packages"

func loadRandomPackage() ([]*packages.Package, error) {
	return packages.Load(&packages.Config{
		Mode: packages.NeedSyntax | packages.NeedTypesInfo | packages.NeedTypes | packages.NeedCompiledGoFiles,
	}, "./testdata/myrandom/...")
}
