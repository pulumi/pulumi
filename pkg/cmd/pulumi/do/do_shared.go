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
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/gofrs/uuid"
	"github.com/hashicorp/hcl/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	hclsyntax "github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pclruntime "github.com/pulumi/pulumi/pkg/v3/pcl/runtime"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	codegenrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/codegen"
)

type functionEvalContext struct {
	WorkingDir    string
	ProjectName   string
	RootDirectory string
}

type inputFlagValue struct {
	value string
	typ   schema.Type
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
	inputFlags map[string]inputFlagValue,
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

	attributeLiterals, err := inputFlagLiterals(inputFlags)
	if err != nil {
		return nil, err
	}
	return evaluatePCL(input, filename, fileType, bind, evalContext, attributeLiterals)
}

func evaluatePCL(
	input io.Reader,
	filename, fileType string,
	bind func(*hclsyntax.File) ([]*model.Attribute, model.Type, []*schema.Property, hcl.Diagnostics),
	evalContext functionEvalContext,
	attributeLiterals map[string]string,
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
	if err := mergeAttributeLiterals(file, filename, fileType, attributeLiterals); err != nil {
		return nil, err
	}

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
// through the named converter plugin's ConvertSnippet RPC and the resulting PCL is fed into the same bind pipeline.
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
	inputFlags map[string]inputFlagValue,
) (resource.PropertyMap, error) {
	if path == "" || inputFormat == "" || inputFormat == "pcl" {
		return evaluatePCLFile(path, fileType, bind, evalContext, inputFlags)
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
	resp, err := converter.ConvertSnippet(ctx, &plugin.ConvertSnippetRequest{
		Filename:     path,
		Source:       source,
		TargetLoader: loaderTarget,
		Package:      packageDescriptor,
		Token:        token,
		Attributes:   inputFlagAttributes(inputFlags),
	})
	if err != nil {
		if status.Code(err) == codes.Unimplemented {
			return nil, fmt.Errorf(
				"%s %s converter does not support snippet conversion; use pcl format or try installing a newer %s converter",
				inputFormat, fileType, inputFormat)
		}
		return nil, fmt.Errorf("generate PCL from %s file: %w", fileType, err)
	}
	if resp.Diagnostics.HasErrors() {
		return nil, resp.Diagnostics
	}
	return evaluatePCL(bytes.NewReader(resp.Source), resp.Filename, fileType, bind, evalContext, resp.Attributes)
}

func evaluateFunctionFile(
	ctx context.Context, path, fileType, inputFormat string, fn *schema.Function, evalContext functionEvalContext,
	loadConverter func(string) (plugin.Converter, error), loaderTarget string,
	packageDescriptor *codegenrpc.GetSchemaRequest,
	inputFlags map[string]inputFlagValue,
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
		inputFlags,
	)
}

func evaluateResourceFile(
	ctx context.Context, path, fileType, inputFormat string, res *schema.Resource, evalContext functionEvalContext,
	loadConverter func(string) (plugin.Converter, error), loaderTarget string,
	packageDescriptor *codegenrpc.GetSchemaRequest,
	inputFlags map[string]inputFlagValue,
	bindOpts ...pcl.BindOption,
) (resource.PropertyMap, error) {
	bind := func(file *hclsyntax.File) ([]*model.Attribute, model.Type, []*schema.Property, hcl.Diagnostics) {
		attrs, inputType, diags := pcl.BindResource(file, res, bindOpts...)
		return attrs, inputType, res.InputProperties, diags
	}
	return evaluateFile(
		ctx, path, fileType, inputFormat, res.Token, bind, loadConverter, loaderTarget, packageDescriptor, evalContext,
		inputFlags,
	)
}

func collectInputFlags(cmd *cobra.Command, namespace string, inputs []*schema.Property) map[string]inputFlagValue {
	values := map[string]inputFlagValue{}
	for _, input := range inputs {
		typ := unwrapType(input.Type)
		if !isInputFlagType(typ) {
			continue
		}

		flagName := inputFlagName(input.Name)
		if flag := cmd.Flag(fmt.Sprintf("%s:%s", namespace, flagName)); flag != nil && flag.Changed {
			values[input.Name] = inputFlagValue{value: flag.Value.String(), typ: typ}
			continue
		}
		if namespace == "input" {
			if flag := cmd.Flag(flagName); flag != nil && flag.Changed {
				values[input.Name] = inputFlagValue{value: flag.Value.String(), typ: typ}
			}
		}
	}
	return values
}

func inputFlagAttributes(inputFlags map[string]inputFlagValue) map[string]string {
	if len(inputFlags) == 0 {
		return nil
	}
	attrs := make(map[string]string, len(inputFlags))
	for name, flag := range inputFlags {
		attrs[name] = flag.value
	}
	return attrs
}

func inputFlagLiterals(inputFlags map[string]inputFlagValue) (map[string]string, error) {
	if len(inputFlags) == 0 {
		return nil, nil
	}
	attrs := make(map[string]string, len(inputFlags))
	for name, flag := range inputFlags {
		literal, err := pclLiteral(flag)
		if err != nil {
			return nil, err
		}
		attrs[name] = literal
	}
	return attrs, nil
}

func mergeAttributeLiterals(
	file *hclsyntax.File, filename, fileType string, attributes map[string]string,
) error {
	if len(attributes) == 0 {
		return nil
	}

	names := make([]string, 0, len(attributes))
	for name := range attributes {
		names = append(names, name)
	}
	sort.Strings(names)

	var overlay strings.Builder
	for _, name := range names {
		fmt.Fprintf(&overlay, "%s = %s\n", name, attributes[name])
	}

	parser := hclsyntax.NewParser()
	overlayName := fmt.Sprintf("%s flags for %s", fileType, filename)
	if err := parser.ParseFile(strings.NewReader(overlay.String()), overlayName); err != nil {
		return fmt.Errorf("parse %s flags: %w", fileType, err)
	}
	if parser.Diagnostics.HasErrors() {
		return parser.Diagnostics
	}
	contract.Assertf(len(parser.Files) == 1, "Should be one PCL flags file")
	for name, attr := range parser.Files[0].Body.Attributes {
		file.Body.Attributes[name] = attr
	}
	return nil
}

func pclLiteral(flag inputFlagValue) (string, error) {
	switch flag.typ {
	case schema.StringType:
		return strconv.Quote(flag.value), nil
	case schema.BoolType, schema.IntType, schema.NumberType:
		return flag.value, nil
	default:
		return "", fmt.Errorf("unsupported flag type %s", flag.typ)
	}
}

func addInputFlags(cmd *cobra.Command, namespace string, inputs []*schema.Property) {
	addInputFlagsTo(cmd, cmd.Flags(), namespace, inputs)
}

func addPersistentInputFlags(cmd *cobra.Command, namespace string, inputs []*schema.Property) {
	addInputFlagsTo(cmd, cmd.PersistentFlags(), namespace, inputs)
}

func addInputFlagsTo(cmd *cobra.Command, flags *pflag.FlagSet, namespace string, inputs []*schema.Property) {
	for _, input := range inputs {
		var flagFunc func(string, string)

		typ := unwrapType(input.Type)

		if typ == schema.StringType {
			flagFunc = func(name, extraHelp string) {
				flags.String(name, "", input.Comment+extraHelp)
			}
		}
		if typ == schema.BoolType {
			flagFunc = func(name, extraHelp string) {
				flags.Bool(name, false, input.Comment+extraHelp)
			}
		}
		if typ == schema.IntType {
			flagFunc = func(name, extraHelp string) {
				flags.Int(name, 0, input.Comment+extraHelp)
			}
		}
		if typ == schema.NumberType {
			flagFunc = func(name, extraHelp string) {
				flags.Float64(name, 0, input.Comment+extraHelp)
			}
		}

		if flagFunc != nil {
			flagName := inputFlagName(input.Name)
			key := fmt.Sprintf("%s:%s", namespace, flagName)
			flagFunc(key, "")
			if namespace == "input" && flags.Lookup(flagName) == nil {
				flagFunc(flagName, " (alias for --"+key+")")
				cmd.MarkFlagsMutuallyExclusive(key, flagName)
				// Mark the namespaced flag as hidden
				flags.Lookup(key).Hidden = true
			}
		}
	}
}

func inputFlagName(name string) string {
	var builder strings.Builder
	var previous rune
	var previousIsWord bool
	var previousIsSeparator bool
	runes := []rune(name)
	for i, r := range runes {
		if r == '_' || r == '-' || unicode.IsSpace(r) {
			if builder.Len() > 0 && !previousIsSeparator {
				builder.WriteRune('-')
			}
			previous = '-'
			previousIsWord = false
			previousIsSeparator = true
			continue
		}

		nextIsLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
		if unicode.IsUpper(r) && previousIsWord &&
			(unicode.IsLower(previous) || unicode.IsDigit(previous) || nextIsLower) {
			builder.WriteRune('-')
		}
		builder.WriteRune(unicode.ToLower(r))
		previous = r
		previousIsWord = true
		previousIsSeparator = false
	}
	return strings.TrimSuffix(builder.String(), "-")
}

func isInputFlagType(typ schema.Type) bool {
	return typ == schema.StringType || typ == schema.BoolType || typ == schema.IntType || typ == schema.NumberType
}

// unwrapType recursively unwraps Optional and Input types to get at the underlying element type.
func unwrapType(typ schema.Type) schema.Type {
	if opt, ok := typ.(*schema.OptionalType); ok {
		return unwrapType(opt.ElementType)
	}
	if input, ok := typ.(*schema.InputType); ok {
		return unwrapType(input.ElementType)
	}
	return typ
}

func resourceURN(res *schema.Resource) resource.URN {
	_, _, name, diags := pcl.DecomposeToken(res.Token, hcl.Range{})
	contract.Assertf(!diags.HasErrors(), "token should decompose")
	return resource.NewURN("dev", "default", "", tokens.Type(res.Token), name)
}

func (pc *packageCommand) configureProvider(cmd *cobra.Command, ctx context.Context) error {
	config, err := evaluateResourceFile(
		ctx, pc.providerFile, "provider", pc.format,
		pc.spec.Provider, pc.evalContext, pc.converter, pc.loaderTarget, pc.packageDescriptor,
		collectInputFlags(cmd, pc.spec.Name, pc.spec.Provider.InputProperties))
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

	return nil
}

// requireYesIfNonInteractive returns ErrNonInteractiveRequiresYes when the user is not on a TTY (so a confirmation
// prompt could never succeed) and --yes was not supplied. Dry-run is exempt since nothing destructive happens.
// This is the same pattern stack rm / package delete / config env init use.
func (pc *packageCommand) requireYesIfNonInteractive(yes bool) error {
	if yes || pc.dryrun {
		return nil
	}
	if !cmdutil.Interactive() {
		return backenderr.ErrNonInteractiveRequiresYes
	}
	return nil
}

// confirm prints summary and asks the user to type confirmName to proceed. The summary and prompt go to stderr so
// that stdout stays a clean JSON channel for piping. Returns nil to proceed; a bail error (suppressed by the
// outer CLI) when the user declines. requireYesIfNonInteractive should have been called earlier; if we somehow
// reach here non-interactively without --yes we treat it as a decline. Uses ui.ConfirmPrompt for the prompt
// itself so the look and feel matches stack rm and friends.
func (pc *packageCommand) confirm(cmd *cobra.Command, summary, confirmName string, yes bool) error {
	stderr := cmd.ErrOrStderr()
	fmt.Fprint(stderr, summary)
	if !strings.HasSuffix(summary, "\n") {
		fmt.Fprintln(stderr)
	}
	if yes || pc.dryrun {
		return nil
	}
	if !cmdutil.Interactive() {
		return backenderr.ErrNonInteractiveRequiresYes
	}
	opts := display.Options{
		Color:  cmdutil.GetGlobalColorization(),
		Stdin:  cmd.InOrStdin(),
		Stdout: stderr,
	}
	if !ui.ConfirmPrompt("", confirmName, opts) {
		return result.FprintBailf(stderr, "confirmation declined")
	}
	return nil
}
