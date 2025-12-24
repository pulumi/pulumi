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
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strings"

	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packages"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/maputil"
	"github.com/spf13/cobra"
)

func newPackageInfoCmd() *cobra.Command {
	var module string
	var resource string
	var function string
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
			pctx, err := plugin.NewContext(cmd.Context(), sink, sink, nil, nil, wd, nil, false, nil)
			if err != nil {
				return err
			}
			defer func() {
				contract.IgnoreError(pctx.Close())
			}()

			if function != "" && resource != "" {
				return errors.New("only one of --function or --resource can be specified")
			}

			parameters := &plugin.ParameterizeArgs{Args: args[1:]}
			spec, _, err := packages.SchemaFromSchemaSource(pctx, args[0], parameters,
				cmdCmd.NewDefaultRegistry(cmd.Context(), pkgWorkspace.Instance, nil, cmdutil.Diag(), env.Global()),
				env.Global(), 0 /* unbounded concurrency */)
			if err != nil {
				return err
			}

			stdout := cmd.OutOrStdout()

			if function != "" {
				return showFunctionInfo(spec, module, function, stdout)
			} else if resource != "" {
				return showResourceInfo(spec, module, resource, stdout)
			} else if module != "" {
				return showModuleInfo(spec, module, stdout)
			}
			return showProviderInfo(spec, args, stdout)
		},
	}

	cmd.Flags().StringVarP(&module, "module", "m", "", "Module name")
	cmd.Flags().StringVarP(&resource, "resource", "r", "", "Resource name")
	cmd.Flags().StringVarP(&function, "function", "f", "", "Function name")

	return cmd
}

func showProviderInfo(spec *schema.PackageSpec, args []string, stdout io.Writer) error {
	contract.Requiref(len(args) > 0, "args", "should be non-empty")

	modules := make(map[string]struct{})
	keys := slices.Concat(
		slices.Collect(maps.Keys(spec.Resources)),
		slices.Collect(maps.Keys(spec.Functions)),
	)
	for _, key := range keys {
		split := strings.Split(key, ":")
		if len(split) < 2 {
			continue
		}
		moduleSplit := strings.Split(split[1], "/")
		modules[moduleSplit[0]] = struct{}{}
	}

	if len(spec.Functions) == 1 {
		for name := range spec.Functions {
			nameSplit := strings.Split(name, ":")
			if len(nameSplit) < 3 {
				return fmt.Errorf("invalid function name %q", name)
			}
			return showFunctionInfo(spec, "", nameSplit[2], stdout)
		}
	}

	if len(spec.Resources) == 1 {
		for name := range spec.Resources {
			nameSplit := strings.Split(name, ":")
			if len(nameSplit) < 3 {
				return fmt.Errorf("invalid resource name %q", name)
			}
			return showResourceInfo(spec, "", nameSplit[2], stdout)
		}
	}

	if len(modules) == 1 {
		for name := range modules {
			return showModuleInfo(spec, name, stdout)
		}
	}

	fmt.Fprintf(stdout, bold("Name")+": %s\n", spec.Name)
	fmt.Fprintf(stdout, bold("Version")+": %s\n", spec.Version)
	fmt.Fprintf(stdout, bold("Description")+": %s\n", summaryFromDescription(spec.Description))
	fmt.Fprintf(stdout, bold("Total resources")+" %d\n", len(spec.Resources))
	fmt.Fprintf(stdout, bold("Total functions")+" %d\n", len(spec.Functions))
	fmt.Fprintf(stdout, bold("Total modules")+": %d\n", len(modules))

	fmt.Fprintln(stdout)

	fmt.Fprintf(stdout, bold("Modules")+": %s\n", strings.Join(maputil.SortedKeys(modules), ", "))

	fmt.Fprintln(stdout)
	strArgs := strings.Join(args, " ")
	moduleString := ""
	if len(modules) > 1 {
		moduleString = "--module <module>"
		fmt.Fprintf(
			stdout,
			"Use 'pulumi package info %s %s' to list resources in a module\n",
			strArgs, moduleString)
	}
	fmt.Fprintf(
		stdout,
		"Use 'pulumi package info %s %s --resource <resource>' for detailed resource info\n",
		strArgs, moduleString)
	return nil
}

func summaryFromDescription(description string) string {
	// The description of a resource is markdown formatted.  We only want to provide a
	// short summary of the description, so we will only show the first paragraph. Note
	// that an empty newline denotes the end of the paragraph, but a regular newline might
	// still be part of the first paragraph, and may be in the middle of a sentence.
	// Therefore we split the description into lines, and join the first paragraph, replacing
	// newlines with spaces.
	var summary strings.Builder
	for _, line := range strings.Split(description, "\n") {
		if strings.TrimSpace(line) == "" {
			break
		}
		summary.WriteString(line + " ")
	}
	return strings.TrimSpace(summary.String())
}

func simplifyModuleName(typ string, name string) (string, error) {
	split := strings.Split(name, ":")
	if len(split) < 3 {
		return "", fmt.Errorf("invalid %s name %q", typ, name)
	}
	moduleSplit := strings.Split(split[1], "/")
	return split[0] + ":" + moduleSplit[0] + ":" + split[2], nil
}

func showModuleInfo(spec *schema.PackageSpec, moduleName string, stdout io.Writer) error {
	fmt.Fprintf(stdout, bold("Name")+": %s\n", spec.Name)
	fmt.Fprintf(stdout, bold("Module")+": %s\n", moduleName)
	fmt.Fprintf(stdout, bold("Version")+": %s\n", spec.Version)
	fmt.Fprintf(stdout, bold("Description")+": %s\n", summaryFromDescription(spec.Description))

	resources := make(map[string]schema.ResourceSpec)
	for res, spec := range spec.Resources {
		simplifiedName, err := simplifyModuleName("resource", res)
		if err != nil {
			return err
		}
		fullModuleName := strings.Split(res, ":")[1]
		split := strings.Split(simplifiedName, ":")
		if len(split) < 3 {
			return fmt.Errorf("invalid resource name %q", res)
		}
		if fullModuleName != moduleName && split[1] != moduleName {
			continue
		}
		resources[split[2]] = spec
	}
	functions := make(map[string]schema.FunctionSpec)
	for fun, spec := range spec.Functions {
		simplifiedName, err := simplifyModuleName("function", fun)
		if err != nil {
			return err
		}
		fullModuleName := strings.Split(fun, ":")[1]
		split := strings.Split(simplifiedName, ":")
		if len(split) < 3 {
			return fmt.Errorf("invalid function name %q", fun)
		}
		if fullModuleName != moduleName && split[1] != moduleName {
			continue
		}
		functions[split[2]] = spec
	}

	if len(resources) == 0 && len(functions) == 0 {
		return fmt.Errorf("module %q not found", moduleName)
	}

	fmt.Fprintf(stdout, bold("Resources")+": %d\n", len(resources))

	fmt.Fprintln(stdout)
	for _, name := range maputil.SortedKeys(resources) {
		fmt.Fprintf(stdout, " - %s: %s\n", bold(name), summaryFromDescription(resources[name].Description))
	}
	fmt.Fprintln(stdout)

	fmt.Fprintf(stdout, bold("Functions")+": %d\n", len(functions))

	fmt.Fprintln(stdout)
	for _, name := range maputil.SortedKeys(functions) {
		fmt.Fprintf(stdout, " - %s: %s\n", bold(name), summaryFromDescription(functions[name].Description))
	}
	return nil
}

func bold(s string) string {
	return colors.Always.Colorize(colors.Bold + s + colors.Reset)
}

func underline(s string) string {
	return colors.Always.Colorize(colors.Underline + s + colors.Reset)
}

func showFunctionInfo(spec *schema.PackageSpec, moduleName, functionName string, stdout io.Writer) error {
	var fun schema.FunctionSpec
	var specFunName string
	if moduleName != "" {
		fullFunctionName := fmt.Sprintf("%s:%s:%s", spec.Name, moduleName, functionName)
		var ok bool
		fun, ok = spec.Functions[fullFunctionName]
		specFunName = fullFunctionName
		if !ok {
			for name, f := range spec.Functions {
				simplifiedName, err := simplifyModuleName("function", name)
				if err != nil {
					return err
				}

				if fullFunctionName == simplifiedName {
					fun = f
					ok = true
					specFunName = name
					break
				}
			}
		}
		if !ok {
			return fmt.Errorf("function %q not found", fullFunctionName)
		}
	} else {
		found := false
		for name, f := range spec.Functions {
			split := strings.Split(name, ":")
			if len(split) < 3 {
				return fmt.Errorf("invalid function name %q", name)
			}
			resName := split[2]
			if resName == functionName {
				if found {
					return fmt.Errorf("ambiguous resource name %q, please use --module <module> to disambiguate", functionName)
				}
				fun = f
				found = true
				specFunName = name
			}
		}
		if !found {
			return fmt.Errorf("function %q not found", functionName)
		}
	}

	fmt.Fprintf(stdout, bold("Function")+": %s\n", specFunName)
	fmt.Fprintf(stdout, bold("Description")+": %s\n", summaryFromDescription(fun.Description))

	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, bold("Inputs")+":")
	hasRequired := false
	for _, name := range maputil.SortedKeys(fun.Inputs.Properties) {
		prop := fun.Inputs.Properties[name]
		requiredStr := ""
		if slices.Contains(fun.Inputs.Required, name) {
			hasRequired = true
			requiredStr = "*"
		}
		typ, err := getType(spec, prop.TypeSpec)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, " - %s (%s%s): %s\n",
			bold(name), underline(typ), underline(requiredStr),
			summaryFromDescription(prop.Description))
	}
	if hasRequired {
		fmt.Fprintf(stdout, "Inputs marked with '*' are required\n")
	}

	var returnType *schema.ReturnTypeSpec
	if fun.ReturnType != nil {
		returnType = fun.ReturnType
	} else if fun.Outputs != nil {
		returnType = &schema.ReturnTypeSpec{
			ObjectTypeSpec: fun.Outputs,
		}
	}
	if returnType != nil {
		fmt.Fprintln(stdout)
		fmt.Fprint(stdout, bold("Outputs")+":")
		if returnType.ObjectTypeSpec != nil {
			fmt.Fprintln(stdout)
			obj := returnType.ObjectTypeSpec
			hasPresent := false
			for _, name := range maputil.SortedKeys(obj.Properties) {
				prop := obj.Properties[name]
				presentStr := ""
				if slices.Contains(obj.Required, name) {
					hasPresent = true
					presentStr = "*"
				}
				typ, err := getType(spec, prop.TypeSpec)
				if err != nil {
					return err
				}
				fmt.Fprintf(stdout, " - %s (%s%s): %s\n",
					bold(name), underline(typ), underline(presentStr),
					summaryFromDescription(prop.Description))
			}
			if hasPresent {
				fmt.Fprintf(stdout, "Outputs marked with '*' are always present\n")
			}
		} else if returnType.TypeSpec != nil {
			typ, err := getType(spec, *returnType.TypeSpec)
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, " %s\n", underline(typ))
		}
	}

	return nil
}

func showResourceInfo(spec *schema.PackageSpec, moduleName, resourceName string, stdout io.Writer) error {
	var res schema.ResourceSpec
	var specResName string
	if moduleName != "" {
		fullResourceName := fmt.Sprintf("%s:%s:%s", spec.Name, moduleName, resourceName)
		var ok bool
		res, ok = spec.Resources[fullResourceName]
		specResName = fullResourceName
		if !ok {
			for name, r := range spec.Resources {
				simplifiedName, err := simplifyModuleName("resource", name)
				if err != nil {
					return err
				}

				if fullResourceName == simplifiedName {
					res = r
					ok = true
					specResName = name
					break
				}
			}
		}
		if !ok {
			return fmt.Errorf("resource %q not found", fullResourceName)
		}
	} else {
		found := false
		for name, r := range spec.Resources {
			split := strings.Split(name, ":")
			if len(split) < 3 {
				return fmt.Errorf("invalid resource name %q", name)
			}
			resName := split[2]
			if resName == resourceName {
				if found {
					return fmt.Errorf("ambiguous resource name %q, please use --module <module> to disambiguate", resourceName)
				}
				res = r
				found = true
				specResName = name
			}
		}
		if !found {
			return fmt.Errorf("resource %q not found", resourceName)
		}
	}

	fmt.Fprintf(stdout, bold("Resource")+": %s\n", specResName)
	fmt.Fprintf(stdout, bold("Description")+": %s\n", summaryFromDescription(res.Description))

	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, bold("Inputs")+":")
	hasRequired := false
	for _, name := range maputil.SortedKeys(res.InputProperties) {
		prop := res.InputProperties[name]
		requiredStr := ""
		if slices.Contains(res.RequiredInputs, name) {
			hasRequired = true
			requiredStr = "*"
		}
		typ, err := getType(spec, prop.TypeSpec)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, " - %s (%s%s): %s\n",
			bold(name), underline(typ), underline(requiredStr),
			summaryFromDescription(prop.Description))
	}
	if hasRequired {
		fmt.Fprintf(stdout, "Inputs marked with '*' are required\n")
	}

	fmt.Fprintln(stdout)

	fmt.Fprintln(stdout, bold("Outputs")+":")
	hasPresent := false
	for _, name := range maputil.SortedKeys(res.Properties) {
		prop := res.Properties[name]
		presentStr := ""
		if slices.Contains(res.Required, name) {
			hasPresent = true
			presentStr = "*"
		}
		typ, err := getType(spec, prop.TypeSpec)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, " - %s (%s%s): %s\n",
			bold(name), underline(typ), underline(presentStr),
			summaryFromDescription(prop.Description))
	}
	if hasPresent {
		fmt.Fprintf(stdout, "Outputs marked with '*' are always present\n")
	}
	return nil
}

func getType(spec *schema.PackageSpec, prop schema.TypeSpec) (string, error) {
	typ := prop.Type
	if typ != "" && typ != "object" && typ != "array" && prop.Ref == "" {
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
	if prop.Type == "object" {
		if prop.AdditionalProperties == nil {
			return "object", nil
		}
		typ, err := getType(spec, *prop.AdditionalProperties)
		if err != nil {
			return "", err
		}
		return "map[string]" + typ, nil
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
				simplifiedName, err := simplifyModuleName("type", ref)
				if err != nil {
					return "", err
				}
				split := strings.Split(simplifiedName, ":")
				return split[2], nil
			}
		}
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
