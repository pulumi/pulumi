package docs

import "strings"

type entryType string

const (
	entryTypeModule   entryType = "module"
	entryTypeResource entryType = "resource"
	entryTypeFunction entryType = "function"
)

type modTreeItem struct {
	Name     string        `json:"name"`
	Type     entryType     `json:"type"`
	Link     string        `json:"link"`
	Children []modTreeItem `json:"children,omitempty"`
}

func (i *modTreeItem) AddItem(newItem modTreeItem) {
	if i.Children == nil {
		i.Children = make([]modTreeItem, 0)
	}
	i.Children = append(i.Children, newItem)
}

func (i *modTreeItem) AddChildItem(newItem modTreeItem) {
	if i.Children == nil {
		i.Children = make([]modTreeItem, 0)
	}
	i.Children = append(i.Children, newItem)
}

var packageTree []modTreeItem

func generatePackageTree(rootMod modContext) error {
	packageTree = make([]modTreeItem, 0)

	for _, m := range rootMod.children {
		name := m.mod
		ti := modTreeItem{
			Name:     name,
			Type:     entryTypeModule,
			Link:     strings.ToLower(name),
			Children: nil,
		}

		packageTree = append(packageTree, ti)
	}

	for _, r := range rootMod.resources {
		name := resourceName(r)
		ti := modTreeItem{
			Name:     name,
			Type:     entryTypeResource,
			Link:     strings.ToLower(name),
			Children: nil,
		}

		packageTree = append(packageTree, ti)
	}

	for _, f := range rootMod.functions {
		name := tokenToName(f.Token)
		ti := modTreeItem{
			Name:     name,
			Type:     entryTypeFunction,
			Link:     strings.ToLower(name),
			Children: nil,
		}

		packageTree = append(packageTree, ti)
	}
	return nil
}
