package docs

import (
	"github.com/pkg/errors"
)

type entryType string

const (
	entryTypeModule   entryType = "module"
	entryTypeResource entryType = "resource"
	entryTypeFunction entryType = "function"
)

// PackageTreeItem is a type for representing a package in a
// navigable tree format starting from the top-level/index/root
// of a package.
type PackageTreeItem struct {
	Name     string            `json:"name"`
	Type     entryType         `json:"type"`
	Link     string            `json:"link"`
	Children []PackageTreeItem `json:"children,omitempty"`
}

func generatePackageTree(rootMod modContext) ([]PackageTreeItem, error) {
	numResources := len(rootMod.resources)
	numFunctions := len(rootMod.functions)
	// +1 to add the module itself as an entry.
	size := numResources + numFunctions + 1
	packageTree := make([]PackageTreeItem, 0, size)

	for _, m := range rootMod.children {
		modName := m.getModuleFileName()
		displayName := modFilenameToDisplayName(modName)

		children, err := generatePackageTree(*m)
		if err != nil {
			return nil, errors.Wrapf(err, "generating children for module %s (mod token: %s)", displayName, m.mod)
		}

		ti := PackageTreeItem{
			Name:     displayName,
			Type:     entryTypeModule,
			Link:     getModuleLink(displayName),
			Children: children,
		}

		packageTree = append(packageTree, ti)
	}

	for _, r := range rootMod.resources {
		name := resourceName(r)
		ti := PackageTreeItem{
			Name:     name,
			Type:     entryTypeResource,
			Link:     getResourceLink(name),
			Children: nil,
		}

		packageTree = append(packageTree, ti)
	}

	for _, f := range rootMod.functions {
		name := tokenToName(f.Token)
		ti := PackageTreeItem{
			Name:     name,
			Type:     entryTypeFunction,
			Link:     getFunctionLink(name),
			Children: nil,
		}

		packageTree = append(packageTree, ti)
	}
	return packageTree, nil
}
