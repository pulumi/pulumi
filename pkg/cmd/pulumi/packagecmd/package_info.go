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
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/schemarender"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
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
				return showFunctionInfo(spec, module, function, stdout)
			} else if resource != "" {
				return showResourceInfo(spec, module, resource, stdout)
			} else if module != "" {
				return showModuleInfo(spec, module, stdout)
			}
			return showProviderInfo(spec, args, stdout)
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

	fmt.Fprintf(stdout, schemarender.Bold("Name")+": %s\n", spec.Name)
	fmt.Fprintf(stdout, schemarender.Bold("Version")+": %s\n", spec.Version)
	fmt.Fprintf(stdout, schemarender.Bold("Description")+": %s\n", schemarender.SummaryFromDescription(spec.Description))
	fmt.Fprintf(stdout, schemarender.Bold("Total resources")+" %d\n", len(spec.Resources))
	fmt.Fprintf(stdout, schemarender.Bold("Total functions")+" %d\n", len(spec.Functions))
	fmt.Fprintf(stdout, schemarender.Bold("Total modules")+": %d\n", len(modules))

	fmt.Fprintln(stdout)

	fmt.Fprintf(stdout, schemarender.Bold("Modules")+": %s\n", strings.Join(maputil.SortedKeys(modules), ", "))

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

func showModuleInfo(spec *schema.PackageSpec, moduleName string, stdout io.Writer) error {
	fmt.Fprintf(stdout, schemarender.Bold("Name")+": %s\n", spec.Name)
	fmt.Fprintf(stdout, schemarender.Bold("Module")+": %s\n", moduleName)
	fmt.Fprintf(stdout, schemarender.Bold("Version")+": %s\n", spec.Version)
	fmt.Fprintf(stdout, schemarender.Bold("Description")+": %s\n", schemarender.SummaryFromDescription(spec.Description))

	resources := make(map[string]schema.ResourceSpec)
	for res, spec := range spec.Resources {
		simplifiedName, err := schemarender.SimplifyModuleName("resource", res)
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
		simplifiedName, err := schemarender.SimplifyModuleName("function", fun)
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

	fmt.Fprintf(stdout, schemarender.Bold("Resources")+": %d\n", len(resources))

	fmt.Fprintln(stdout)
	for _, name := range maputil.SortedKeys(resources) {
		fmt.Fprintf(stdout, " - %s: %s\n", schemarender.Bold(name), schemarender.SummaryFromDescription(resources[name].Description))
	}
	fmt.Fprintln(stdout)

	fmt.Fprintf(stdout, schemarender.Bold("Functions")+": %d\n", len(functions))

	fmt.Fprintln(stdout)
	for _, name := range maputil.SortedKeys(functions) {
		fmt.Fprintf(stdout, " - %s: %s\n", schemarender.Bold(name), schemarender.SummaryFromDescription(functions[name].Description))
	}
	return nil
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
				simplifiedName, err := schemarender.SimplifyModuleName("function", name)
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

	fmt.Fprintf(stdout, schemarender.Bold("Function")+": %s\n", specFunName)
	fmt.Fprintf(stdout, schemarender.Bold("Description")+": %s\n", schemarender.SummaryFromDescription(fun.Description))

	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, schemarender.Bold("Inputs")+":")
	hasRequired := false
	for _, name := range maputil.SortedKeys(fun.Inputs.Properties) {
		prop := fun.Inputs.Properties[name]
		requiredStr := ""
		if slices.Contains(fun.Inputs.Required, name) {
			hasRequired = true
			requiredStr = "*"
		}
		typ, err := schemarender.GetType(spec, prop.TypeSpec)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, " - %s (%s%s): %s\n",
			schemarender.Bold(name), schemarender.Underline(typ), schemarender.Underline(requiredStr),
			schemarender.SummaryFromDescription(prop.Description))
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
		fmt.Fprint(stdout, schemarender.Bold("Outputs")+":")
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
				typ, err := schemarender.GetType(spec, prop.TypeSpec)
				if err != nil {
					return err
				}
				fmt.Fprintf(stdout, " - %s (%s%s): %s\n",
					schemarender.Bold(name), schemarender.Underline(typ), schemarender.Underline(presentStr),
					schemarender.SummaryFromDescription(prop.Description))
			}
			if hasPresent {
				fmt.Fprintf(stdout, "Outputs marked with '*' are always present\n")
			}
		} else if returnType.TypeSpec != nil {
			typ, err := schemarender.GetType(spec, *returnType.TypeSpec)
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, " %s\n", schemarender.Underline(typ))
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
				simplifiedName, err := schemarender.SimplifyModuleName("resource", name)
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

	fmt.Fprintf(stdout, schemarender.Bold("Resource")+": %s\n", specResName)
	fmt.Fprintf(stdout, schemarender.Bold("Description")+": %s\n", schemarender.SummaryFromDescription(res.Description))

	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, schemarender.Bold("Inputs")+":")
	hasRequired := false
	for _, name := range maputil.SortedKeys(res.InputProperties) {
		prop := res.InputProperties[name]
		requiredStr := ""
		if slices.Contains(res.RequiredInputs, name) {
			hasRequired = true
			requiredStr = "*"
		}
		typ, err := schemarender.GetType(spec, prop.TypeSpec)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, " - %s (%s%s): %s\n",
			schemarender.Bold(name), schemarender.Underline(typ), schemarender.Underline(requiredStr),
			schemarender.SummaryFromDescription(prop.Description))
	}
	if hasRequired {
		fmt.Fprintf(stdout, "Inputs marked with '*' are required\n")
	}

	fmt.Fprintln(stdout)

	fmt.Fprintln(stdout, schemarender.Bold("Outputs")+":")
	hasPresent := false
	for _, name := range maputil.SortedKeys(res.Properties) {
		prop := res.Properties[name]
		presentStr := ""
		if slices.Contains(res.Required, name) {
			hasPresent = true
			presentStr = "*"
		}
		typ, err := schemarender.GetType(spec, prop.TypeSpec)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, " - %s (%s%s): %s\n",
			schemarender.Bold(name), schemarender.Underline(typ), schemarender.Underline(presentStr),
			schemarender.SummaryFromDescription(prop.Description))
	}
	if hasPresent {
		fmt.Fprintf(stdout, "Outputs marked with '*' are always present\n")
	}
	return nil
}
