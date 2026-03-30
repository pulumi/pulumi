// Copyright 2026, Pulumi Corporation.
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

package registry

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/blang/semver"
	"github.com/dustin/go-humanize"
	cmdcmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/schemarender"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	commonregistry "github.com/pulumi/pulumi/sdk/v3/go/common/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/spf13/cobra"
)

type packageListItemJSON struct {
	Name          string   `json:"name"`
	Publisher     string   `json:"publisher"`
	Source        string   `json:"source"`
	Version       string   `json:"version"`
	Title         string   `json:"title,omitempty"`
	Description   string   `json:"description,omitempty"`
	Category      string   `json:"category,omitempty"`
	PackageTypes  []string `json:"packageTypes,omitempty"`
	PackageStatus string   `json:"packageStatus"`
	Visibility    string   `json:"visibility"`
}

func newRegistryLsCmd() *cobra.Command {
	var jsonOut bool
	var pkgType string
	var visibility string
	var sortBy string
	var search string

	c := &cobra.Command{
		Use:   "ls [name]",
		Short: "List packages",
		Long: `List packages in the Pulumi Registry.

Optionally filter by exact package name. Use --search for keyword matching
across names, descriptions, and categories. Use --type and --visibility
to narrow results. Use --sort-by to control ordering.

Note: version listing for a specific package is not yet supported by the
registry API. Use 'registry package get <name> --version <ver>' to fetch
a specific version.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			reg := cmdcmd.NewDefaultRegistry(ctx, pkgWorkspace.Instance, nil, cmdutil.Diag(), env.Global())

			// If a specific package name is given, try to resolve it and list all versions.
			if len(args) > 0 {
				meta, err := commonregistry.ResolvePackageFromName(ctx, reg, args[0], nil)
				if err == nil {
					return listPackageVersions(ctx, reg, meta, jsonOut)
				}
				// If resolution fails, fall through to listing packages by name filter.
			}

			var nameFilter *string
			if len(args) > 0 {
				nameFilter = &args[0]
			}

			var results []apitype.PackageMetadata
			query := strings.ToLower(search)
			for meta, err := range reg.ListPackages(ctx, nameFilter) {
				if err != nil {
					return fmt.Errorf("listing packages: %w", err)
				}
				if query != "" && !matchesQuery(meta, query) {
					continue
				}
				results = append(results, meta)
			}

			results = deduplicatePackages(results)
			results = filterPackages(results, pkgType, visibility)
			sortPackages(results, sortBy)

			if jsonOut {
				return formatPackageListJSON(results)
			}

			if cmdutil.Interactive() {
				listItems := make([]registryItem, len(results))
				for i, p := range results {
					listItems[i] = registryItem{
						title: fmt.Sprintf("%-30s  %-12s  %s  %-20s  %s",
							p.Name, formatVersion(p.Version), formatKindShort(p.PackageTypes),
							p.Publisher, schemarender.SummaryFromDescription(p.Description)),
						filterText: p.Name,
						value:      i,
					}
				}
				idx, err := runInteractiveList(
					fmt.Sprintf("%d packages", len(results)),
					fmt.Sprintf("%-30s  %-12s  %s  %-20s  %s", "NAME", "VERSION", "K", "PUBLISHER", "DESCRIPTION"),
					listItems)
				if err != nil {
					return err
				}
				if idx >= 0 {
					fmt.Println()
					return browsePackage(ctx, reg, results[idx])
				}
				return nil
			}

			return formatPackageListConsole(results)
		},
	}

	constrictor.AttachArguments(c, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "name"},
		},
	})

	c.PersistentFlags().BoolVarP(&jsonOut, "json", "j", false, "Emit output as JSON")
	c.PersistentFlags().StringVarP(&search, "search", "s", "",
		"Search keyword (matches against name, description, and category)")
	c.PersistentFlags().StringVar(&pkgType, "type", "", "Filter by package type (native, component, bridged)")
	c.PersistentFlags().StringVar(&visibility, "visibility", "all", "Filter by visibility (public, private, @org, all)")
	c.PersistentFlags().StringVar(&sortBy, "sort-by", "name", "Sort results by field (name, publisher, version)")

	return c
}

func filterPackages(packages []apitype.PackageMetadata, pkgType, visibility string) []apitype.PackageMetadata {
	var result []apitype.PackageMetadata
	for _, p := range packages {
		if visibility != "" && visibility != "all" {
			if strings.HasPrefix(visibility, "@") {
				// Filter to private packages owned by a specific org, e.g. "@pulumi".
				org := strings.TrimPrefix(visibility, "@")
				if p.Visibility != apitype.VisibilityPrivate || p.Publisher != org {
					continue
				}
			} else if visibility == "public" && p.Visibility != apitype.VisibilityPublic {
				continue
			} else if visibility == "private" && p.Visibility != apitype.VisibilityPrivate {
				continue
			}
		}
		if pkgType != "" {
			found := false
			for _, t := range p.PackageTypes {
				if string(t) == pkgType {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		result = append(result, p)
	}
	return result
}

func listPackageVersions(
	ctx context.Context,
	reg commonregistry.Registry,
	meta apitype.PackageMetadata,
	jsonOut bool,
) error {
	var versions []apitype.PackageMetadata
	for v, err := range reg.ListPackageVersions(ctx, meta.Source, meta.Publisher, meta.Name) {
		if err != nil {
			return fmt.Errorf("listing versions: %w", err)
		}
		versions = append(versions, v)
	}

	if jsonOut {
		return formatPackageListJSON(versions)
	}

	if cmdutil.Interactive() {
		listItems := make([]registryItem, len(versions))
		for i, v := range versions {
			listItems[i] = registryItem{
				title: fmt.Sprintf("%-20s  %-8s  %s", formatVersion(v.Version), v.PackageStatus.String(), humanize.Time(v.CreatedAt)),
				value: i,
			}
		}
		idx, err := runInteractiveList(
			fmt.Sprintf("%s — %d versions", meta.Name, len(versions)),
			fmt.Sprintf("%-20s  %-8s  %s", "VERSION", "STATUS", "PUBLISHED"),
			listItems)
		if err != nil {
			return err
		}
		if idx >= 0 {
			fmt.Println()
			return formatPackageInfoConsole(versions[idx], true) // version detail, no navigation
		}
		return nil
	}

	rows := make([]cmdutil.TableRow, len(versions))
	for i, v := range versions {
		published := humanize.Time(v.CreatedAt)
		rows[i] = cmdutil.TableRow{
			Columns: []string{formatVersion(v.Version), v.PackageStatus.String(), published},
		}
	}

	fmt.Printf("%s (%s/%s)\n\n", meta.Name, meta.Source, meta.Publisher)
	ui.PrintTable(cmdutil.Table{
		Headers: []string{"VERSION", "STATUS", "PUBLISHED"},
		Rows:    rows,
	}, nil)
	fmt.Printf("\nTotal: %d version(s)\n", len(versions))
	return nil
}

func matchesQuery(meta apitype.PackageMetadata, query string) bool {
	return strings.Contains(strings.ToLower(meta.Name), query) ||
		strings.Contains(strings.ToLower(meta.Description), query) ||
		strings.Contains(strings.ToLower(meta.Category), query)
}

// formatKindShort returns a single-letter kind indicator: P=provider, C=component.
func formatKindShort(types []apitype.PackageType) string {
	hasComponent := false
	for _, t := range types {
		if t == apitype.PackageTypeComponent {
			hasComponent = true
		}
	}
	if hasComponent {
		return "C"
	}
	return "P"
}

// formatVersion formats a version for table display, truncating long pre-release strings.
func formatVersion(v semver.Version) string {
	s := v.String()
	if len(s) > 20 {
		// Truncate long pre-release versions like "0.0.0-xa9b2fca01dbb067cc4..."
		return s[:17] + "..."
	}
	return s
}

// formatOwner returns "public" for public packages, or "@org" for private ones.
func formatOwner(p apitype.PackageMetadata) string {
	if p.Visibility == apitype.VisibilityPrivate {
		return "@" + p.Publisher
	}
	return "public"
}

// deduplicatePackages keeps only one entry per package name.
// When multiple publishers have the same package, prefer the one with the latest version.
// If versions are equal, prefer the "pulumi" publisher as the canonical source.
func deduplicatePackages(packages []apitype.PackageMetadata) []apitype.PackageMetadata {
	best := make(map[string]apitype.PackageMetadata)
	for _, p := range packages {
		existing, ok := best[p.Name]
		if !ok {
			best[p.Name] = p
			continue
		}
		// Prefer higher version.
		if p.Version.GT(existing.Version) {
			best[p.Name] = p
		} else if p.Version.EQ(existing.Version) && p.Publisher == "pulumi" && existing.Publisher != "pulumi" {
			// Same version — prefer the pulumi publisher.
			best[p.Name] = p
		}
	}
	result := make([]apitype.PackageMetadata, 0, len(best))
	for _, p := range best {
		result = append(result, p)
	}
	return result
}

func sortPackages(packages []apitype.PackageMetadata, sortBy string) {
	sort.Slice(packages, func(i, j int) bool {
		switch sortBy {
		case "publisher":
			if packages[i].Publisher != packages[j].Publisher {
				return packages[i].Publisher < packages[j].Publisher
			}
			return packages[i].Name < packages[j].Name
		case "version":
			return packages[i].Version.GT(packages[j].Version)
		default: // "name"
			return packages[i].Name < packages[j].Name
		}
	})
}

func formatPackageListJSON(packages []apitype.PackageMetadata) error {
	items := make([]packageListItemJSON, len(packages))
	for i, p := range packages {
		var types []string
		for _, t := range p.PackageTypes {
			types = append(types, string(t))
		}
		items[i] = packageListItemJSON{
			Name:          p.Name,
			Publisher:     p.Publisher,
			Source:        p.Source,
			Version:       p.Version.String(),
			Title:         p.Title,
			Description:   p.Description,
			Category:      p.Category,
			PackageTypes:  types,
			PackageStatus: p.PackageStatus.String(),
			Visibility:    p.Visibility.String(),
		}
	}
	return ui.PrintJSON(items)
}

func formatPackageListConsole(packages []apitype.PackageMetadata) error {
	if len(packages) == 0 {
		fmt.Println("No packages found")
		return nil
	}

	rows := make([]cmdutil.TableRow, len(packages))
	for i, p := range packages {
		rows[i] = cmdutil.TableRow{
			Columns: []string{p.Name, formatVersion(p.Version), formatOwner(p), p.Publisher},
		}
	}

	ui.PrintTable(cmdutil.Table{
		Headers: []string{"NAME", "VERSION", "OWNER", "PUBLISHER"},
		Rows:    rows,
	}, nil)

	fmt.Printf("\nTotal: %d packages\n", len(packages))
	return nil
}

