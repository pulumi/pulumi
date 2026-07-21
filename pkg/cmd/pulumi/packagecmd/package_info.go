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
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strings"

	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packages"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageworkspace"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/schemainfo"
	"github.com/pulumi/pulumi/pkg/v3/codegen/convert"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pkghost "github.com/pulumi/pulumi/pkg/v3/host"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
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
			registry := cmdCmd.NewDefaultRegistry(
				cmd.Context(), cmdBackend.DefaultLoginManager, pkgWorkspace.Instance, nil, sink, env.Global())
			pluginHost, err := pkghost.New(context.WithoutCancel(cmd.Context()), sink, sink, nil,
				pkgWorkspace.EnsureLanguageInstalled, schema.NewLoaderServerFromContext, convert.NewMapperServerFromContext,
				packageworkspace.NewResolverServer(registry))
			if err != nil {
				return err
			}
			// host is owned here, closed after the context
			defer contract.IgnoreClose(pluginHost)
			pctx, err := plugin.NewContext(cmd.Context(), sink, sink, pluginHost, nil, wd, nil, false,
				nil)
			if err != nil {
				return err
			}
			defer contract.IgnoreClose(pctx)

			if function != "" && resource != "" {
				return errors.New("only one of --function or --resource can be specified")
			}

			parameters := &plugin.ParameterizeArgs{Args: args[1:]}
			stdout := cmd.OutOrStdout()
			color := cmdutil.GetGlobalColorization()

			loadPartial := func() (*schema.PartialPackage, error) {
				return packages.PartialPackageFromSchemaSource(cmd.Context(), pkgWorkspace.Instance, pctx, args[0],
					parameters, registry, env.Global(), 0 /* unbounded concurrency */)
			}

			if function != "" {
				pp, err := loadPartial()
				if err != nil {
					return err
				}
				return showFunctionInfo(pp, module, function, stdout, color)
			} else if resource != "" {
				pp, err := loadPartial()
				if err != nil {
					return err
				}
				return showResourceInfo(pp, module, resource, stdout, color)
			}

			spec, _, err := packages.SchemaFromSchemaSource(pkgWorkspace.Instance, pctx, args[0], parameters,
				registry, env.Global(), 0 /* unbounded concurrency */)
			if err != nil {
				return err
			}
			if module != "" {
				return showModuleInfo(spec, module, stdout, color)
			}
			return showProviderInfo(spec, loadPartial, args, stdout, color)
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

func showProviderInfo(
	spec *schema.PackageSpec, loadPartial func() (*schema.PartialPackage, error), args []string, stdout io.Writer,
	color colors.Colorization,
) error {
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
			pp, err := loadPartial()
			if err != nil {
				return err
			}
			return showFunctionInfo(pp, "", nameSplit[2], stdout, color)
		}
	}

	if len(spec.Resources) == 1 {
		for name := range spec.Resources {
			nameSplit := strings.Split(name, ":")
			if len(nameSplit) < 3 {
				return fmt.Errorf("invalid resource name %q", name)
			}
			pp, err := loadPartial()
			if err != nil {
				return err
			}
			return showResourceInfo(pp, "", nameSplit[2], stdout, color)
		}
	}

	if len(modules) == 1 {
		for name := range modules {
			return showModuleInfo(spec, name, stdout, color)
		}
	}

	bold := func(s string) string { return schemainfo.Bold(color, s) }
	fmt.Fprintf(stdout, bold("Name")+": %s\n", spec.Name)
	fmt.Fprintf(stdout, bold("Version")+": %s\n", spec.Version)
	fmt.Fprintf(stdout, bold("Description")+": %s\n", schemainfo.Summarize(spec.Description))
	fmt.Fprintf(stdout, bold("Total resources")+" %d\n", len(spec.Resources))
	fmt.Fprintf(stdout, bold("Total functions")+" %d\n", len(spec.Functions))
	fmt.Fprintf(stdout, bold("Total modules")+": %d\n", len(modules))

	fmt.Fprintln(stdout)

	fmt.Fprintf(stdout, bold("Modules")+": %s\n", strings.Join(slices.Sorted(maps.Keys(modules)), ", "))

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

// writeDescription renders the full schema description below a Description label, inline when it
// fits on one line.
func writeDescription(stdout io.Writer, color colors.Colorization, comment string) {
	description := schemainfo.RenderDescription(comment)
	if description == "" {
		return
	}
	label := schemainfo.Bold(color, "Description")
	if strings.Contains(description, "\n") {
		fmt.Fprintf(stdout, label+":\n%s\n", description)
	} else {
		fmt.Fprintf(stdout, label+": %s\n", description)
	}
}

func simplifyModuleName(typ string, name string) (string, error) {
	split := strings.Split(name, ":")
	if len(split) < 3 {
		return "", fmt.Errorf("invalid %s name %q", typ, name)
	}
	moduleSplit := strings.Split(split[1], "/")
	return split[0] + ":" + moduleSplit[0] + ":" + split[2], nil
}

func showModuleInfo(spec *schema.PackageSpec, moduleName string, stdout io.Writer, color colors.Colorization) error {
	bold := func(s string) string { return schemainfo.Bold(color, s) }
	fmt.Fprintf(stdout, bold("Name")+": %s\n", spec.Name)
	fmt.Fprintf(stdout, bold("Module")+": %s\n", moduleName)
	fmt.Fprintf(stdout, bold("Version")+": %s\n", spec.Version)
	fmt.Fprintf(stdout, bold("Description")+": %s\n", schemainfo.Summarize(spec.Description))

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
	for _, name := range slices.Sorted(maps.Keys(resources)) {
		fmt.Fprintf(stdout, " - %s: %s\n", bold(name), schemainfo.Summarize(resources[name].Description))
	}
	fmt.Fprintln(stdout)

	fmt.Fprintf(stdout, bold("Functions")+": %d\n", len(functions))

	fmt.Fprintln(stdout)
	for _, name := range slices.Sorted(maps.Keys(functions)) {
		fmt.Fprintf(stdout, " - %s: %s\n", bold(name), schemainfo.Summarize(functions[name].Description))
	}
	return nil
}

func showFunctionInfo(
	pp *schema.PartialPackage, moduleName, functionName string, stdout io.Writer, color colors.Colorization,
) error {
	var memberTokens []string
	for it := pp.Functions().Range(); it.Next(); {
		memberTokens = append(memberTokens, it.Token())
	}
	token, err := findMemberToken(pp, memberTokens, "function", moduleName, functionName)
	if err != nil {
		return err
	}
	fun, ok, err := pp.Functions().Get(token)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("function %q not found", functionName)
	}

	bold := func(s string) string { return schemainfo.Bold(color, s) }
	fmt.Fprintf(stdout, bold("Function")+": %s\n", fun.Token)
	writeDescription(stdout, color, fun.Comment)

	fmt.Fprintln(stdout)
	var inputProperties []*schema.Property
	if fun.Inputs != nil {
		inputProperties = fun.Inputs.Properties
	}
	schemainfo.WriteProperties(stdout, color, "Inputs", schemainfo.BoundProperties(inputProperties), schemainfo.Inputs)

	// A bound function's object outputs live in Outputs; a single non-object return value lives in
	// ReturnType and renders inline.
	if fun.Outputs != nil {
		fmt.Fprintln(stdout)
		outputs := schemainfo.BoundProperties(fun.Outputs.Properties)
		schemainfo.WriteProperties(stdout, color, "Outputs", outputs, schemainfo.Outputs)
	} else if fun.ReturnType != nil {
		fmt.Fprintln(stdout)
		fmt.Fprintf(stdout, bold("Outputs")+": %s\n",
			schemainfo.Underline(color, schemainfo.TypeString(fun.ReturnType)))
	}

	return nil
}

func showResourceInfo(
	pp *schema.PartialPackage, moduleName, resourceName string, stdout io.Writer, color colors.Colorization,
) error {
	var memberTokens []string
	for it := pp.Resources().Range(); it.Next(); {
		memberTokens = append(memberTokens, it.Token())
	}
	token, err := findMemberToken(pp, memberTokens, "resource", moduleName, resourceName)
	if err != nil {
		return err
	}
	res, ok, err := pp.Resources().Get(token)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("resource %q not found", resourceName)
	}

	bold := func(s string) string { return schemainfo.Bold(color, s) }
	fmt.Fprintf(stdout, bold("Resource")+": %s\n", res.Token)
	writeDescription(stdout, color, res.Comment)

	fmt.Fprintln(stdout)
	schemainfo.WriteProperties(
		stdout, color, "Inputs", schemainfo.BoundProperties(res.InputProperties), schemainfo.Inputs)

	fmt.Fprintln(stdout)
	schemainfo.WriteProperties(stdout, color, "Outputs", schemainfo.BoundProperties(res.Properties), schemainfo.Outputs)
	return nil
}

// findMemberToken resolves the full token of a resource or function from its unqualified name,
// optionally disambiguated by module.
func findMemberToken(pp *schema.PartialPackage, memberTokens []string, kind, moduleName, name string) (string, error) {
	var found string
	for _, token := range memberTokens {
		if !tokens.Token(token).HasModuleMember() || string(tokens.Type(token).Name()) != name {
			continue
		}
		if moduleName != "" {
			if tokenModule(pp, token) == moduleName {
				return token, nil
			}
			continue
		}
		if found != "" {
			return "", fmt.Errorf("ambiguous %s name %q, please use --module <module> to disambiguate", kind, name)
		}
		found = token
	}
	if found == "" {
		return "", fmt.Errorf("%s %q not found", kind, name)
	}
	return found, nil
}

// tokenModule returns the module of a Pulumi token per the package's module format, mapping the
// root module back to its "index" spelling so it can be matched against --module index.
func tokenModule(pp *schema.PartialPackage, token string) string {
	if module := pp.TokenToModule(token); module != "" {
		return module
	}
	return "index"
}
