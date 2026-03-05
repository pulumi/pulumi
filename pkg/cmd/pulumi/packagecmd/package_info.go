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
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packages"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
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
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "info",
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
			pctx, err := plugin.NewContext(cmd.Context(), sink, sink, nil, nil, wd, nil, false,
				nil, schema.NewLoaderServerFromHost)
			if err != nil {
				return err
			}
			defer contract.IgnoreClose(pctx)

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
				return showFunctionInfo(spec, module, function, stdout, jsonOut)
			} else if resource != "" {
				return showResourceInfo(spec, module, resource, stdout, jsonOut)
			} else if module != "" {
				return showModuleInfo(spec, module, stdout, jsonOut)
			}
			return showProviderInfo(spec, args, stdout, jsonOut)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "provider", Usage: "<provider|schema|path>"},
			{Name: "provider-parameter"},
		},
		Required: 1,
		Variadic: true,
	})

	// It's worth mentioning the `--`, as it means that Cobra will stop parsing flags.
	// In other words, a provider parameter can be `--foo` as long as it's after `--`.
	cmd.Use = "info <provider|schema|path> [flags] [--] [provider-parameter]..."

	cmd.Flags().StringVarP(&module, "module", "m", "", "Module name")
	cmd.Flags().StringVarP(&resource, "resource", "r", "", "Resource name")
	cmd.Flags().StringVarP(&function, "function", "f", "", "Function name")
	cmd.Flags().BoolVarP(&jsonOut, "json", "j", false, "Emit output as JSON")

	return cmd
}

// Data types used for both JSON serialization and text rendering.

type providerInfo struct {
	Name           string   `json:"name"`
	Version        string   `json:"version"`
	Description    string   `json:"description"`
	TotalResources int      `json:"totalResources"`
	TotalFunctions int      `json:"totalFunctions"`
	Modules        []string `json:"modules"`
}

func (p *providerInfo) writeText(w io.Writer, args []string) {
	fmt.Fprintf(w, bold("Name")+": %s\n", p.Name)
	fmt.Fprintf(w, bold("Version")+": %s\n", p.Version)
	fmt.Fprintf(w, bold("Description")+": %s\n", p.Description)
	fmt.Fprintf(w, bold("Total resources")+" %d\n", p.TotalResources)
	fmt.Fprintf(w, bold("Total functions")+" %d\n", p.TotalFunctions)
	fmt.Fprintf(w, bold("Total modules")+": %d\n", len(p.Modules))

	fmt.Fprintln(w)

	fmt.Fprintf(w, bold("Modules")+": %s\n", strings.Join(p.Modules, ", "))

	fmt.Fprintln(w)
	strArgs := strings.Join(args, " ")
	moduleString := ""
	if len(p.Modules) > 1 {
		moduleString = "--module <module>"
		fmt.Fprintf(w,
			"Use 'pulumi package info %s %s' to list resources in a module\n",
			strArgs, moduleString)
	}
	fmt.Fprintf(w,
		"Use 'pulumi package info %s %s --resource <resource>' for detailed resource info\n",
		strArgs, moduleString)
}

type moduleInfo struct {
	Name        string                  `json:"name"`
	Module      string                  `json:"module"`
	Version     string                  `json:"version"`
	Description string                  `json:"description"`
	Resources   map[string]moduleMember `json:"resources"`
	Functions   map[string]moduleMember `json:"functions"`
}

type moduleMember struct {
	Description        string `json:"description"`
	DeprecationMessage string `json:"deprecationMessage,omitempty"`
}

func (m *moduleInfo) writeText(w io.Writer) {
	fmt.Fprintf(w, bold("Name")+": %s\n", m.Name)
	fmt.Fprintf(w, bold("Module")+": %s\n", m.Module)
	fmt.Fprintf(w, bold("Version")+": %s\n", m.Version)
	fmt.Fprintf(w, bold("Description")+": %s\n", m.Description)

	fmt.Fprintf(w, bold("Resources")+": %d\n", len(m.Resources))
	fmt.Fprintln(w)
	for _, name := range maputil.SortedKeys(m.Resources) {
		fmt.Fprintf(w, " - %s: %s\n", bold(name), m.Resources[name].Description)
	}
	fmt.Fprintln(w)

	fmt.Fprintf(w, bold("Functions")+": %d\n", len(m.Functions))
	fmt.Fprintln(w)
	for _, name := range maputil.SortedKeys(m.Functions) {
		fmt.Fprintf(w, " - %s: %s\n", bold(name), m.Functions[name].Description)
	}
}

type resourceInfo struct {
	Resource           string                  `json:"resource"`
	Description        string                  `json:"description"`
	DeprecationMessage string                  `json:"deprecationMessage,omitempty"`
	Inputs             map[string]propertyInfo `json:"inputs"`
	Outputs            map[string]propertyInfo `json:"outputs"`
}

func (r *resourceInfo) writeText(w io.Writer) {
	fmt.Fprintf(w, bold("Resource")+": %s\n", r.Resource)
	fmt.Fprintf(w, bold("Description")+": %s\n", r.Description)

	fmt.Fprintln(w)
	writePropertyMap(w, "Inputs", "required", r.Inputs)
	fmt.Fprintln(w)
	writePropertyMap(w, "Outputs", "always present", r.Outputs)
}

type functionInfo struct {
	Function           string                  `json:"function"`
	Description        string                  `json:"description"`
	DeprecationMessage string                  `json:"deprecationMessage,omitempty"`
	Inputs             map[string]propertyInfo `json:"inputs"`
	Outputs            *functionOutput         `json:"outputs,omitempty"`
}

type functionOutput struct {
	Type       string                  `json:"type,omitempty"`
	Properties map[string]propertyInfo `json:"properties,omitempty"`
}

func (f *functionInfo) writeText(w io.Writer) {
	fmt.Fprintf(w, bold("Function")+": %s\n", f.Function)
	fmt.Fprintf(w, bold("Description")+": %s\n", f.Description)

	fmt.Fprintln(w)
	writePropertyMap(w, "Inputs", "required", f.Inputs)

	if f.Outputs != nil {
		fmt.Fprintln(w)
		fmt.Fprint(w, bold("Outputs")+":")
		if len(f.Outputs.Properties) > 0 {
			fmt.Fprintln(w)
			writePropertyMapBody(w, "Outputs", "always present", f.Outputs.Properties)
		} else if f.Outputs.Type != "" {
			fmt.Fprintf(w, " %s\n", underline(f.Outputs.Type))
		}
	}
}

type propertyInfo struct {
	Type               string `json:"type"`
	Description        string `json:"description"`
	Required           bool   `json:"required"`
	DeprecationMessage string `json:"deprecationMessage,omitempty"`
}

// writePropertyMap writes a labeled property list (e.g. "Inputs:" or "Outputs:") with
// a legend for the required marker.
func writePropertyMap(w io.Writer, label, requiredLegend string, props map[string]propertyInfo) {
	fmt.Fprintln(w, bold(label)+":")
	writePropertyMapBody(w, label, requiredLegend, props)
}

// writePropertyMapBody writes the property list body without the header label.
func writePropertyMapBody(w io.Writer, label, requiredLegend string, props map[string]propertyInfo) {
	hasRequired := false
	for _, name := range maputil.SortedKeys(props) {
		prop := props[name]
		requiredStr := ""
		if prop.Required {
			hasRequired = true
			requiredStr = "*"
		}
		fmt.Fprintf(w, " - %s (%s%s): %s\n",
			bold(name), underline(prop.Type), underline(requiredStr),
			prop.Description)
	}
	if hasRequired {
		fmt.Fprintf(w, "%s marked with '*' are %s\n", label, requiredLegend)
	}
}

// Show functions: lookup → build struct → JSON or text output.

func showProviderInfo(spec *schema.PackageSpec, args []string, stdout io.Writer, jsonOut bool) error {
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

	// For text output, automatically drill down when there's only one
	// function/resource/module. For JSON output, always return the
	// provider-level shape so consumers get a predictable schema.
	if !jsonOut {
		if len(spec.Functions) == 1 {
			for name := range spec.Functions {
				nameSplit := strings.Split(name, ":")
				if len(nameSplit) < 3 {
					return fmt.Errorf("invalid function name %q", name)
				}
				return showFunctionInfo(spec, "", nameSplit[2], stdout, jsonOut)
			}
		}

		if len(spec.Resources) == 1 {
			for name := range spec.Resources {
				nameSplit := strings.Split(name, ":")
				if len(nameSplit) < 3 {
					return fmt.Errorf("invalid resource name %q", name)
				}
				return showResourceInfo(spec, "", nameSplit[2], stdout, jsonOut)
			}
		}

		if len(modules) == 1 {
			for name := range modules {
				return showModuleInfo(spec, name, stdout, jsonOut)
			}
		}
	}

	info := providerInfo{
		Name:           spec.Name,
		Version:        spec.Version,
		Description:    summaryFromDescription(spec.Description),
		TotalResources: len(spec.Resources),
		TotalFunctions: len(spec.Functions),
		Modules:        maputil.SortedKeys(modules),
	}

	if jsonOut {
		return ui.FprintJSON(stdout, info)
	}
	info.writeText(stdout, args)
	return nil
}

func showModuleInfo(spec *schema.PackageSpec, moduleName string, stdout io.Writer, jsonOut bool) error {
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

	infoResources := make(map[string]moduleMember, len(resources))
	for name, res := range resources {
		infoResources[name] = moduleMember{
			Description:        summaryFromDescription(res.Description),
			DeprecationMessage: res.DeprecationMessage,
		}
	}
	infoFunctions := make(map[string]moduleMember, len(functions))
	for name, fun := range functions {
		infoFunctions[name] = moduleMember{
			Description:        summaryFromDescription(fun.Description),
			DeprecationMessage: fun.DeprecationMessage,
		}
	}

	info := moduleInfo{
		Name:        spec.Name,
		Module:      moduleName,
		Version:     spec.Version,
		Description: summaryFromDescription(spec.Description),
		Resources:   infoResources,
		Functions:   infoFunctions,
	}

	if jsonOut {
		return ui.FprintJSON(stdout, info)
	}
	info.writeText(stdout)
	return nil
}

func showFunctionInfo(
	spec *schema.PackageSpec, moduleName, functionName string, stdout io.Writer, jsonOut bool,
) error {
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

	info, err := buildFunctionInfo(spec, &fun, specFunName)
	if err != nil {
		return err
	}
	if jsonOut {
		return ui.FprintJSON(stdout, info)
	}
	info.writeText(stdout)
	return nil
}

func buildFunctionInfo(
	spec *schema.PackageSpec, fun *schema.FunctionSpec, specFunName string,
) (*functionInfo, error) {
	inputs, err := buildPropertyInfoMap(spec, fun.Inputs.Properties, fun.Inputs.Required)
	if err != nil {
		return nil, err
	}

	info := &functionInfo{
		Function:           specFunName,
		Description:        summaryFromDescription(fun.Description),
		DeprecationMessage: fun.DeprecationMessage,
		Inputs:             inputs,
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
		if returnType.ObjectTypeSpec != nil {
			outputs, err := buildPropertyInfoMap(
				spec, returnType.ObjectTypeSpec.Properties, returnType.ObjectTypeSpec.Required)
			if err != nil {
				return nil, err
			}
			info.Outputs = &functionOutput{Properties: outputs}
		} else if returnType.TypeSpec != nil {
			typ, err := getType(spec, *returnType.TypeSpec)
			if err != nil {
				return nil, err
			}
			info.Outputs = &functionOutput{Type: typ}
		}
	}

	return info, nil
}

func showResourceInfo(
	spec *schema.PackageSpec, moduleName, resourceName string, stdout io.Writer, jsonOut bool,
) error {
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

	info, err := buildResourceInfo(spec, &res, specResName)
	if err != nil {
		return err
	}
	if jsonOut {
		return ui.FprintJSON(stdout, info)
	}
	info.writeText(stdout)
	return nil
}

func buildResourceInfo(
	spec *schema.PackageSpec, res *schema.ResourceSpec, specResName string,
) (*resourceInfo, error) {
	inputs, err := buildPropertyInfoMap(spec, res.InputProperties, res.RequiredInputs)
	if err != nil {
		return nil, err
	}
	outputs, err := buildPropertyInfoMap(spec, res.Properties, res.Required)
	if err != nil {
		return nil, err
	}
	return &resourceInfo{
		Resource:           specResName,
		Description:        summaryFromDescription(res.Description),
		DeprecationMessage: res.DeprecationMessage,
		Inputs:             inputs,
		Outputs:            outputs,
	}, nil
}

// Helpers.

func buildPropertyInfoMap(
	spec *schema.PackageSpec,
	properties map[string]schema.PropertySpec,
	required []string,
) (map[string]propertyInfo, error) {
	result := make(map[string]propertyInfo, len(properties))
	for name, prop := range properties {
		typ, err := getType(spec, prop.TypeSpec)
		if err != nil {
			return nil, err
		}
		result[name] = propertyInfo{
			Type:               typ,
			Description:        summaryFromDescription(prop.Description),
			Required:           slices.Contains(required, name),
			DeprecationMessage: prop.DeprecationMessage,
		}
	}
	return result, nil
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

func bold(s string) string {
	return colors.Always.Colorize(colors.Bold + s + colors.Reset)
}

func underline(s string) string {
	return colors.Always.Colorize(colors.Underline + s + colors.Reset)
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
