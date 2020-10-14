package provider

import (
	"fmt"

	"golang.org/x/tools/go/packages"
)

func loadTestPackage(name string) (string, []*packages.Package, error) {
	thisPkg, err := packages.Load(nil, ".")
	if err != nil || len(thisPkg) == 0 {
		return "", nil, fmt.Errorf("failed to load test utilities package: %v", err)
	}

	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedSyntax | packages.NeedTypesInfo | packages.NeedTypes | packages.NeedCompiledGoFiles,
	}, fmt.Sprintf("./testdata/%s/...", name))
	if err != nil || len(pkgs) == 0 {
		return "", nil, fmt.Errorf("failed to load test package: %v", err)
	}
	return thisPkg[0].PkgPath + "/testdata/" + name, pkgs, err
}

func loadRandomPackage() (string, []*packages.Package, error) {
	return loadTestPackage("myrandom")
}

func loadLandscapePackage() (string, []*packages.Package, error) {
	return loadTestPackage("landscape")
}
