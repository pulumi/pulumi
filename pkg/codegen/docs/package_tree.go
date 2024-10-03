// Copyright 2021-2024, Pulumi Corporation.
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
	"fmt"
	"sort"

	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
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
	packageTree := slice.Prealloc[PackageTreeItem](size)

	conflictResolver := rootMod.docGenContext.newModuleConflictResolver()

	for _, m := range rootMod.children {
		modName := m.getModuleFileName()
		displayName := modFilenameToDisplayName(modName)

		children, err := generatePackageTree(*m)
		if err != nil {
			return nil, fmt.Errorf("generating children for module %s (mod token: %s): %w", displayName, m.mod, err)
		}

		safeName := conflictResolver.getSafeName(displayName, m)
		if safeName == "" {
			continue // unresolved conflict
		}
		ti := PackageTreeItem{
			Name:     displayName,
			Type:     entryTypeModule,
			Link:     getModuleLink(safeName),
			Children: children,
		}

		packageTree = append(packageTree, ti)
	}
	sort.Slice(packageTree, func(i, j int) bool {
		return packageTree[i].Name < packageTree[j].Name
	})

	for _, r := range rootMod.resources {
		name := resourceName(r)
		safeName := conflictResolver.getSafeName(name, r)
		if safeName == "" {
			continue // unresolved conflict
		}
		ti := PackageTreeItem{
			Name:     name,
			Type:     entryTypeResource,
			Link:     getResourceLink(safeName),
			Children: nil,
		}

		packageTree = append(packageTree, ti)
	}
	sort.SliceStable(packageTree, func(i, j int) bool {
		pti, ptj := packageTree[i], packageTree[j]
		switch {
		case pti.Type != ptj.Type:
			return pti.Type == entryTypeModule && ptj.Type != entryTypeModule
		default:
			return pti.Name < ptj.Name
		}
	})

	for _, f := range rootMod.functions {
		name := tokenToName(f.Token)
		safeName := conflictResolver.getSafeName(name, f)
		if safeName == "" {
			continue // unresolved conflict
		}
		ti := PackageTreeItem{
			Name:     name,
			Type:     entryTypeFunction,
			Link:     getFunctionLink(safeName),
			Children: nil,
		}

		packageTree = append(packageTree, ti)
	}
	sort.SliceStable(packageTree, func(i, j int) bool {
		pti, ptj := packageTree[i], packageTree[j]
		switch {
		case pti.Type != ptj.Type:
			return (pti.Type == entryTypeModule || pti.Type == entryTypeResource) &&
				(ptj.Type != entryTypeModule && ptj.Type != entryTypeResource)
		default:
			return pti.Name < ptj.Name
		}
	})
	return packageTree, nil
}
