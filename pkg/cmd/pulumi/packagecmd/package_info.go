// Copyright 2025, Pulumi Corporation.
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

package packagecmd

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/maputil"
	"github.com/spf13/cobra"
)

func newPackageInfoCmd() *cobra.Command {
	var module string
	var resource string
	cmd := &cobra.Command{
		Use:   "info <provider|schema|path> [provider-parameter...]",
		Args:  cmdutil.MinimumNArgs(1),
		Short: "Show information about a package",
		Long: `Show information about a package

This command shows information about a package, its modules and detailed resource info.

The <provider> argument can be specified in the same way as in 'pulumi package add'.
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			sink := cmdutil.Diag()
			pctx, err := plugin.NewContext(sink, sink, nil, nil, wd, nil, false, nil)
			if err != nil {
				return err
			}
			defer func() {
				contract.IgnoreError(pctx.Close())
			}()

			pkg, err := SchemaFromSchemaSource(pctx, args[0], args[1:])
			if err != nil {
				return err
			}
			spec, err := pkg.MarshalSpec()
			if err != nil {
				return err
			}

			stdout := cmd.OutOrStdout()

			if resource != "" && module == "" {
				return fmt.Errorf("resource name %q specified without module", resource)
			}

			if module == "" {
				showProviderInfo(spec, args, stdout)
			} else if resource == "" {
				return showModuleInfo(spec, module, stdout)
			} else {
				return showResourceInfo(spec, module, resource, stdout)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&module, "module", "m", "", "Module name")
	cmd.Flags().StringVarP(&resource, "resource", "r", "", "Resource name")

	return cmd
}

func showProviderInfo(spec *schema.PackageSpec, args []string, stdout io.Writer) {
	fmt.Fprintf(stdout, "Provider: %s (%s)\n", spec.Name, spec.Version)
	fmt.Fprintf(stdout, "  Description: %s\n", summaryFromDescription(spec.Description))
	fmt.Fprintf(stdout, "Total resources: %d\n", len(spec.Resources))

	modules := make(map[string]struct{})
	for res := range spec.Resources {
		split := strings.Split(res, ":")
		if len(split) < 2 {
			continue
		}
		moduleSplit := strings.Split(split[1], "/")
		modules[moduleSplit[0]] = struct{}{}
	}
	fmt.Fprintf(stdout, "Total modules: %d\n", len(modules))

	fmt.Fprintln(stdout)
	fmt.Fprintf(
		stdout,
		"Use 'pulumi package info --module <module> %s' to list resources in a module\n",
		strings.Join(args, " "))
	fmt.Fprintf(
		stdout,
		"Use 'pulumi package info --module <module> --resource <resource> %s' for detailed resource info\n",
		strings.Join(args, " "))
}

func summaryFromDescription(description string) string {
	// The description of a resource is markdown formatted.  We only want to provide aa
	// short summary of the description, so we will only show the first paragraph. Note
	// that an empty newline denotes the end of the paragraph, but a regular newline might
	// still be part of the first paragraph, and may be in the middle of a sentence.
	// Therefore we split the description into lines, and join the first paragraph, replacing
	// newlines with spaces.
	summary := ""
	for _, line := range strings.Split(description, "\n") {
		if strings.TrimSpace(line) == "" {
			break
		}
		summary += line + " "
	}
	return strings.TrimSpace(summary)
}

func simplifyModuleName(resourceName string) (string, error) {
	split := strings.Split(resourceName, ":")
	if len(split) < 3 {
		return "", fmt.Errorf("invalid resource name %q", resourceName)
	}
	moduleSplit := strings.Split(split[1], "/")
	return split[0] + ":" + moduleSplit[0] + ":" + split[2], nil
}

func showModuleInfo(spec *schema.PackageSpec, moduleName string, stdout io.Writer) error {
	fmt.Fprintf(stdout, "Module: %s:%s\n", spec.Name, moduleName)

	resources := make(map[string]schema.ResourceSpec)
	for res, spec := range spec.Resources {
		simplifiedName, err := simplifyModuleName(res)
		if err != nil {
			return err
		}
		split := strings.Split(simplifiedName, ":")
		if len(split) < 3 {
			return fmt.Errorf("invalid resource name %q", res)
		}
		if split[1] != moduleName {
			continue
		}
		resources[split[2]] = spec
	}

	if len(resources) == 0 {
		return fmt.Errorf("module %q not found", moduleName)
	}
	fmt.Fprintf(stdout, "Resources: %d\n", len(resources))

	fmt.Fprintln(stdout)
	for _, name := range maputil.SortedKeys(resources) {
		fmt.Fprintf(stdout, " - %s: %s\n", name, summaryFromDescription(resources[name].Description))
	}
	return nil
}

func showResourceInfo(spec *schema.PackageSpec, moduleName, resourceName string, stdout io.Writer) error {
	fullResourceName := fmt.Sprintf("%s:%s:%s", spec.Name, moduleName, resourceName)

	fmt.Fprintf(stdout, "Resource: %s\n", fullResourceName)

	res, ok := spec.Resources[fullResourceName]
	if !ok {
		for name, r := range spec.Resources {
			simplifiedName, err := simplifyModuleName(name)
			if err != nil {
				return err
			}
			if fullResourceName == simplifiedName {
				res = r
				ok = true
				break
			}
		}
	}
	if !ok {
		return fmt.Errorf("resource %q not found", fullResourceName)
	}

	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Input Properties:")
	for _, name := range maputil.SortedKeys(res.InputProperties) {
		prop := res.InputProperties[name]
		optionalStr := ""
		if !slices.Contains(res.RequiredInputs, name) {
			optionalStr = " (optional)"
		}
		typ, err := getType(spec, prop.TypeSpec)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, " - %s (%s%s): %s\n",
			name, typ, optionalStr, summaryFromDescription(prop.Description))
	}

	fmt.Fprintln(stdout)

	fmt.Fprintln(stdout, "Output Properties:")
	for _, name := range maputil.SortedKeys(res.Properties) {
		prop := res.Properties[name]
		presentStr := ""
		if slices.Contains(res.Required, name) {
			presentStr = " (always present)"
		}
		typ, err := getType(spec, prop.TypeSpec)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, " - %s (%s%s): %s\n",
			name, typ, presentStr, summaryFromDescription(prop.Description))
	}
	return nil
}

func getType(spec *schema.PackageSpec, prop schema.TypeSpec) (string, error) {
	typ := prop.Type
	if typ != "" && typ != "object" && typ != "array" {
		return typ, nil
	}
	if prop.Type == "array" {
		if prop.Items == nil {
			return "[]unknown", nil
		}
		typ, err := getType(spec, *prop.Items)
		if err != nil {
			return "", err
		}
		return "[]" + typ, nil
	}
	if prop.Ref != "" {
		if strings.HasPrefix(prop.Ref, "#/types/") {
			ref := strings.TrimPrefix(prop.Ref, "#/types/")
			ref = strings.ReplaceAll(ref, "%2F", "/")
			if typeSpec, ok := spec.Types[ref]; ok {
				if len(typeSpec.Enum) > 0 {
					return fmt.Sprintf("enum(%s){%s}",
						typeSpec.Type, formatEnumValues(typeSpec.Enum)), nil
				}
				simplifiedName, err := simplifyModuleName(ref)
				if err != nil {
					return "", err
				}
				split := strings.Split(simplifiedName, ":")
				return split[2], nil
			}
		}
		return prop.Ref, nil
	}
	if prop.Ref != "" {
		return prop.Ref, nil
	}
	return "unknown", nil
}

func formatEnumValues(enum []schema.EnumValueSpec) string {
	var values []string
	for _, v := range enum {
		if v.Name != "" {
			values = append(values, v.Name)
		} else if v.Value != nil {
			values = append(values, fmt.Sprintf("%v", v.Value))
		}
	}
	return strings.Join(values, ", ")
}
