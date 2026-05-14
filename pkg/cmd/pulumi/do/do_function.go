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

package do

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gofrs/uuid"
	"github.com/hashicorp/hcl/v2"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	hclsyntax "github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pclruntime "github.com/pulumi/pulumi/pkg/v3/pcl/runtime"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	codegenrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/codegen"
)

type functionEvalContext struct {
	WorkingDir    string
	ProjectName   string
	RootDirectory string
}

func jsonifyPropertyValue(v resource.PropertyValue, showSecrets bool) (any, error) {
	if !showSecrets && (v.IsSecret() || (v.IsOutput() && v.OutputValue().Secret)) {
		return "[secret]", nil
	}

	if v.IsComputed() || (v.IsOutput() && !v.OutputValue().Known) {
		return "[unknown]", nil
	}

	if v.IsSecret() {
		return jsonifyPropertyValue(v.SecretValue().Element, showSecrets)
	}

	if v.IsOutput() {
		return jsonifyPropertyValue(v.OutputValue().Element, showSecrets)
	}

	if v.IsAsset() {
		return v.AssetValue().Serialize(), nil
	}

	if v.IsArchive() {
		return v.ArchiveValue().Serialize(), nil
	}

	if v.IsArray() {
		arr := v.ArrayValue()
		res := make([]any, len(arr))
		for i := range arr {
			ev, err := jsonifyPropertyValue(arr[i], showSecrets)
			if err != nil {
				return nil, err
			}
			res[i] = ev
		}
		return res, nil
	}

	if v.IsObject() {
		obj := v.ObjectValue()
		res := make(map[string]any, len(obj))
		for k, v := range obj {
			ev, err := jsonifyPropertyValue(v, showSecrets)
			if err != nil {
				return nil, err
			}
			res[string(k)] = ev
		}
		return res, nil
	}

	return v.V, nil
}

// jsonifyProperty converts a Property to a JSON string for display purposes. This strips things like secrets and
// outputs down to their underlying values.
func jsonifyProperty(prop resource.PropertyValue, showSecrets bool) (string, error) {
	plain, err := jsonifyPropertyValue(prop, showSecrets)
	if err != nil {
		return "", err
	}

	json, err := ui.MakeJSONString(plain, true)
	if err != nil {
		return "", err
	}
	return json, nil
}

// filterOutputs recursively filters the given property map to only include properties present in the schema. This is
// used to filter out any "internal" keys the provider might return that aren't part of the declared outputs, which
// would otherwise cause the JSON output to be noisy and potentially break consumers expecting a specific shape.
func filterOutputs(props resource.PropertyMap, properties []*schema.Property) resource.PropertyMap {
	filtered := resource.PropertyMap{}
	for _, property := range properties {
		key := resource.PropertyKey(property.Name)
		if value, ok := props[key]; ok {
			filtered[key] = filterOutput(value, property.Type)
		}
	}
	return filtered
}

func filterOutput(
	prop resource.PropertyValue, typ schema.Type,
) resource.PropertyValue {
	if typ == nil {
		return prop
	}

	if optTyp, ok := typ.(*schema.OptionalType); ok {
		if prop.IsNull() {
			return resource.NewNullProperty()
		} else {
			typ = optTyp.ElementType
		}
	}

	isSecret := prop.IsSecret()
	if isSecret {
		return resource.MakeSecret(filterOutput(prop.SecretValue().Element, typ))
	}

	switch t := typ.(type) {
	case *schema.ObjectType:
		if prop.IsObject() {
			return resource.NewProperty(filterOutputs(prop.ObjectValue(), t.Properties))
		}
	case *schema.ArrayType:
		if prop.IsArray() {
			arr := prop.ArrayValue()
			filtered := make([]resource.PropertyValue, len(arr))
			for i, el := range arr {
				filtered[i] = filterOutput(el, t.ElementType)
			}
			return resource.NewProperty(filtered)
		}
	case *schema.MapType:
		if prop.IsObject() {
			obj := prop.ObjectValue()
			filtered := make(resource.PropertyMap, len(obj))
			for k, v := range obj {
				filtered[k] = filterOutput(v, t.ElementType)
			}
			return resource.NewProperty(filtered)
		}
	case *schema.UnionType:
		// Pick the first variant whose shape matches the runtime value. Discriminated unions would let us be more
		// precise, but most cases here are simple kind-based dispatch.
		for _, elt := range t.ElementTypes {
			if unionVariantMatches(prop, elt) {
				return filterOutput(prop, elt)
			}
		}
	}

	return prop
}

// unionVariantMatches reports whether the schema type is structurally compatible with the runtime kind of prop.
// Used to pick a union variant for output filtering.
func unionVariantMatches(prop resource.PropertyValue, typ schema.Type) bool {
	// TODO https://github.com/pulumi/pulumi/issues/23234: This needs to be smarter and handle Discriminator
	if opt, ok := typ.(*schema.OptionalType); ok {
		typ = opt.ElementType
	}
	switch typ.(type) {
	case *schema.ObjectType, *schema.MapType:
		return prop.IsObject()
	case *schema.ArrayType:
		return prop.IsArray()
	}
	return false
}

// evaluatePCLFile reads, binds, and evaluates a PCL input file against a caller-supplied schema. The bind callback
// decides how the parsed file is type-checked (function vs. resource) and returns the schema property list used to
// coerce values during evaluation.
func evaluatePCLFile(
	path, fileType string,
	bind func(*hclsyntax.File) ([]*model.Attribute, model.Type, []*schema.Property, hcl.Diagnostics),
	evalContext functionEvalContext,
) (resource.PropertyMap, error) {
	// When no input file is supplied we still run the bind step against an empty file so that the schema's
	// required-input check fires.
	var input io.Reader
	filename := path
	if path == "" {
		input = strings.NewReader("")
		filename = fmt.Sprintf("<no %s file>", fileType)
	} else {
		f, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("open %s file: %w", fileType, err)
		}
		defer contract.IgnoreClose(f)
		input = f
	}

	return evaluatePCL(input, filename, fileType, bind, evalContext)
}

func evaluatePCL(
	input io.Reader,
	filename, fileType string,
	bind func(*hclsyntax.File) ([]*model.Attribute, model.Type, []*schema.Property, hcl.Diagnostics),
	evalContext functionEvalContext,
) (resource.PropertyMap, error) {
	parser := hclsyntax.NewParser()
	if err := parser.ParseFile(input, filename); err != nil {
		return nil, fmt.Errorf("parse input: %w", err)
	}
	if parser.Diagnostics.HasErrors() {
		return nil, parser.Diagnostics
	}
	contract.Assertf(len(parser.Files) == 1, "Should be one PCL file")
	file := parser.Files[0]

	attrs, inputType, properties, diagnostics := bind(file)
	if diagnostics.HasErrors() {
		return nil, diagnostics
	}

	notSupported := func(what string) error {
		return fmt.Errorf("cannot %s in %s file", what, fileType)
	}
	ectx := pclruntime.NewEvalContext(
		evalContext.WorkingDir,
		evalContext.RootDirectory,
		"",
		evalContext.ProjectName,
		"",
		func(context.Context, string) (*schema.Resource, error) {
			return nil, notSupported("reference resources")
		},
		func(context.Context, string) (*schema.Function, error) {
			return nil, notSupported("reference functions")
		},
		func(context.Context, resource.ResourceReference) (resource.PropertyMap, error) {
			return nil, notSupported("reference resources")
		},
		func(context.Context, *pulumirpc.ResourceInvokeRequest) (*pulumirpc.InvokeResponse, error) {
			return nil, notSupported("invoke functions")
		},
		func(context.Context, *pulumirpc.ResourceCallRequest) (*pulumirpc.CallResponse, error) {
			return nil, notSupported("call functions")
		},
	)

	result, poison, diags := ectx.EvaluateObject(attrs, inputType, properties)
	if poison != nil {
		// `pulumi do` is one-shot — there's no upstream resource graph to propagate poison through, so surface it.
		return nil, fmt.Errorf("%s file references unknown resource %s", fileType, *poison)
	}
	if diags.HasErrors() {
		return nil, diags
	}
	return result, nil
}

// evaluateFile reads an input file in the given format and evaluates it. For non-PCL formats the source is routed
// through the named converter plugin's GenerateSnippet RPC and the resulting PCL is fed into the same bind pipeline.
// An empty path is treated as "no input provided" and always goes through the PCL path so the bind step's
// missing-required check still fires.
func evaluateFile(
	ctx context.Context,
	path, fileType, inputFormat, token string,
	bind func(*hclsyntax.File) ([]*model.Attribute, model.Type, []*schema.Property, hcl.Diagnostics),
	loadConverter func(string) (plugin.Converter, error),
	loaderTarget string,
	packageDescriptor *codegenrpc.GetSchemaRequest,
	evalContext functionEvalContext,
) (resource.PropertyMap, error) {
	if path == "" || inputFormat == "" || inputFormat == "pcl" {
		return evaluatePCLFile(path, fileType, bind, evalContext)
	}

	converter, err := loadConverter(inputFormat)
	if err != nil {
		return nil, fmt.Errorf("load %s input converter: %w", inputFormat, err)
	}
	defer contract.IgnoreClose(converter)

	source, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s file: %w", fileType, err)
	}
	resp, err := converter.GenerateSnippet(ctx, &plugin.GenerateSnippetRequest{
		Filename:     path,
		Source:       source,
		TargetLoader: loaderTarget,
		Package:      packageDescriptor,
		Token:        token,
	})
	if err != nil {
		return nil, fmt.Errorf("generate PCL from %s file: %w", fileType, err)
	}
	if resp.Diagnostics.HasErrors() {
		return nil, resp.Diagnostics
	}
	return evaluatePCL(bytes.NewReader(resp.Source), resp.Filename, fileType, bind, evalContext)
}

func evaluateFunctionFile(
	ctx context.Context, path, fileType, inputFormat string, fn *schema.Function, evalContext functionEvalContext,
	loadConverter func(string) (plugin.Converter, error), loaderTarget string,
	packageDescriptor *codegenrpc.GetSchemaRequest,
) (resource.PropertyMap, error) {
	bind := func(file *hclsyntax.File) ([]*model.Attribute, model.Type, []*schema.Property, hcl.Diagnostics) {
		attrs, inputType, diags := pcl.BindFunction(file, fn)
		var properties []*schema.Property
		if fn.Inputs != nil {
			properties = fn.Inputs.Properties
		}
		return attrs, inputType, properties, diags
	}
	return evaluateFile(
		ctx, path, fileType, inputFormat, fn.Token, bind, loadConverter, loaderTarget, packageDescriptor, evalContext,
	)
}

func evaluateResourceFile(
	ctx context.Context, path, fileType, inputFormat string, res *schema.Resource, evalContext functionEvalContext,
	loadConverter func(string) (plugin.Converter, error), loaderTarget string,
	packageDescriptor *codegenrpc.GetSchemaRequest,
) (resource.PropertyMap, error) {
	bind := func(file *hclsyntax.File) ([]*model.Attribute, model.Type, []*schema.Property, hcl.Diagnostics) {
		attrs, inputType, diags := pcl.BindResource(file, res)
		return attrs, inputType, res.InputProperties, diags
	}
	return evaluateFile(
		ctx, path, fileType, inputFormat, res.Token, bind, loadConverter, loaderTarget, packageDescriptor, evalContext,
	)
}

func functionSchemaHelp(fn *schema.Function) string {
	var b strings.Builder
	writeProperties := func(title string, properties []*schema.Property, includeRequired bool) {
		if len(properties) == 0 {
			return
		}
		if b.Len() > 0 {
			trimmed := strings.TrimSuffix(b.String(), "\n")
			b.Reset()
			b.WriteString(trimmed)
			b.WriteString("\n\n")
		}
		b.WriteString(title)
		b.WriteString(":\n")
		for _, property := range properties {
			fmt.Fprintf(&b, "  %s (%s", property.Name, functionTypeString(property.Type))
			if includeRequired {
				if property.IsRequired() {
					b.WriteString(", required")
				} else {
					b.WriteString(", optional")
				}
			}
			b.WriteString(")")
			if property.Comment != "" {
				fmt.Fprintf(&b, " - %s", strings.ReplaceAll(property.Comment, "\n", " "))
			}
			b.WriteString("\n")
		}
	}

	if fn.Inputs != nil {
		writeProperties("Inputs", fn.Inputs.Properties, true)
	}
	if fn.Outputs != nil {
		writeProperties("Outputs", fn.Outputs.Properties, true)
	} else if fn.ReturnType != nil {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		fmt.Fprintf(&b, "Outputs:\n  result (%s)\n", functionTypeString(fn.ReturnType))
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func functionTypeString(t schema.Type) string {
	switch t := t.(type) {
	case *schema.OptionalType:
		return functionTypeString(t.ElementType)
	case *schema.InputType:
		return functionTypeString(t.ElementType)
	default:
		return t.String()
	}
}

func (pc *packageCommand) newFunctionCommand(fn *schema.Function) *cobra.Command {
	_, _, name, diags := pcl.DecomposeToken(fn.Token, hcl.Range{})
	contract.Assertf(!diags.HasErrors(), "token should decompose")

	shorthelp := fmt.Sprintf("Invoke the %s function", name)
	longhelp := shorthelp + "."
	if fn.Comment != "" {
		longhelp = fmt.Sprintf("%s\n\n%s", longhelp, fn.Comment)
	}
	if schemaHelp := functionSchemaHelp(fn); schemaHelp != "" {
		longhelp = fmt.Sprintf("%s\n\n%s", longhelp, schemaHelp)
	}

	var inputFile string
	var inputFormat string

	cmd := &cobra.Command{
		Use:     name,
		GroupID: "Functions",
		Short:   shorthelp,
		Long:    longhelp,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// We need to evaluate the provider configuration so we can call Configure on the provider before invoking
			// the function.
			config, err := evaluateResourceFile(
				ctx, pc.providerFile, "provider", pc.providerFormat,
				pc.spec.Provider, pc.evalContext, pc.converter, pc.loaderTarget, pc.packageDescriptor)
			if err != nil {
				return fmt.Errorf("parse provider file: %w", err)
			}

			urn := resource.NewURN("dev", "default", "", tokens.Type("pulumi:providers:"+pc.spec.Name), "")
			name := urn.Name()
			typ := urn.Type()
			uuid, err := uuid.NewV4()
			if err != nil {
				return fmt.Errorf("generate UUID: %w", err)
			}
			id := resource.ID(uuid.String())

			_, err = pc.provider.Configure(ctx, plugin.ConfigureRequest{
				URN:    &urn,
				Name:   &name,
				Type:   &typ,
				ID:     &id,
				Inputs: config,
			})
			if err != nil {
				return fmt.Errorf("configure provider: %w", err)
			}

			inputs, err := evaluateFunctionFile(
				cmd.Context(), inputFile, "input", inputFormat, fn, pc.evalContext,
				pc.converter, pc.loaderTarget, pc.packageDescriptor)
			if err != nil {
				return fmt.Errorf("parse input file: %w", err)
			}

			response, err := pc.provider.Invoke(ctx, plugin.InvokeRequest{
				Tok:     tokens.ModuleMember(fn.Token),
				Args:    inputs,
				Preview: pc.dryrun,
			})
			if err != nil {
				return err
			}

			var result resource.PropertyValue
			if fn.Outputs != nil {
				result = resource.NewProperty(filterOutputs(response.Properties, fn.Outputs.Properties))
			} else if fn.ReturnType != nil {
				if len(response.Properties) != 1 {
					return fmt.Errorf("expected exactly one return value from function but got %d", len(response.Properties))
				}

				for _, value := range response.Properties {
					result = filterOutput(value, fn.ReturnType)
					break
				}
			}

			// Print the response as JSON to stdout.
			outputs, err := jsonifyProperty(result, pc.showSecrets)
			if err != nil {
				return fmt.Errorf("failed to convert outputs to JSON: %w", err)
			}

			fmt.Fprint(cmd.OutOrStdout(), outputs)
			return nil
		},
	}

	cmd.Flags().StringVar(&inputFormat, "input", "pcl", "Input file format")
	cmd.Flags().StringVar(&inputFile, "input-file", "", "Path to a file containing function inputs")

	return cmd
}
