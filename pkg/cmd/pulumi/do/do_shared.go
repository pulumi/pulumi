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
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/gofrs/uuid"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	backendSecrets "github.com/pulumi/pulumi/pkg/v3/backend/secrets"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	hclsyntax "github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pclruntime "github.com/pulumi/pulumi/pkg/v3/pcl/runtime"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	codegenrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/codegen"
)

func startSpinner(prefix string) func() {
	spinner, ticker := cmdutil.NewSpinnerAndTicker(
		prefix, nil, cmdutil.GetGlobalColorization(), 8 /*timesPerSecond*/, !cmdutil.Interactive(),
	)
	spinner.Tick()
	stop := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		for {
			select {
			case <-ticker.C:
				spinner.Tick()
			case <-stop:
				spinner.Reset()
				return
			}
		}
	}()
	var once sync.Once
	return func() {
		once.Do(func() {
			ticker.Stop()
			close(stop)
			<-stopped
		})
	}
}

type functionEvalContext struct {
	WorkingDir    string
	ProjectName   string
	RootDirectory string
	Organization  string
	Stack         string
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
		evalContext.Organization,
		evalContext.ProjectName,
		evalContext.Stack,
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
// An empty path with no input flags is treated as "no input provided"; converter-backed formats still run for
// resource/function input flags so those flags are interpreted by the selected converter before the bind step's
// missing-required check runs.
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
	contract.Requiref(inputFormat != "", "inputFormat", "inputFormat must be non-empty")
	filename := path

	var pcl []byte
	if path == "" {
		// Bind still runs against an empty file so the schema's required-input check fires.
		filename = fmt.Sprintf("<no %s file>", fileType)
	} else {
		var err error
		pcl, err = os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("open %s file: %w", fileType, err)
		}
	}

	var literals map[string]string
	if len(pcl) == 0 && len(inputFlags) == 0 {
		// No input provided; pcl and literals are empty
	} else if inputFormat == "pcl" {
		var err error
		literals, err = inputFlagLiterals(inputFlags)
		if err != nil {
			return nil, err
		}
	} else {
		converter, err := loadConverter(inputFormat)
		if err != nil {
			return nil, fmt.Errorf("load %s input converter: %w", inputFormat, err)
		}
		defer contract.IgnoreClose(converter)

		resp, err := converter.ConvertSnippet(ctx, &plugin.ConvertSnippetRequest{
			Filename:     filename,
			Source:       pcl,
			TargetLoader: loaderTarget,
			Package:      packageDescriptor,
			Token:        token,
			Attributes:   inputFlagAttributes(inputFlags),
		})
		if err != nil {
			if status.Code(err) == codes.Unimplemented {
				return nil, fmt.Errorf(
					"%s %s converter does not support snippet conversion; use pcl format or try installing a newer %s converter",
					inputFormat, fileType, inputFormat,
				)
			}
			return nil, fmt.Errorf("generate PCL from %s file: %w", fileType, err)
		}
		if resp.Diagnostics.HasErrors() {
			return nil, resp.Diagnostics
		}
		pcl = resp.Source
		filename = resp.Filename
		if filename == "" {
			filename = fmt.Sprintf("<converted %s>", fileType)
		}
		literals = resp.Attributes
	}

	merged, err := mergeAttributeLiteralsIntoPCL(pcl, filename, fileType, literals)
	if err != nil {
		return nil, err
	}
	return evaluatePCL(bytes.NewReader(merged), filename, fileType, bind, evalContext)
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

// mergeAttributeLiteralsIntoPCL merges `name = literal` attribute assignments into source at the
// top level and returns the resulting PCL bytes. Each entry in attrs is a name and a serialized
// PCL literal (e.g. `"foo"`, `42`, `true`) — the same shape converter plugins return from
// ConvertSnippet and that inputFlagLiterals produces for --input-* flags. Uses hclwrite so an
// existing attribute of the same name is replaced in place rather than duplicated, and non-flag
// content (blocks, comments, formatting) survives the round trip.
func mergeAttributeLiteralsIntoPCL(
	source []byte, filename, fileType string, attrs map[string]string,
) ([]byte, error) {
	if len(attrs) == 0 {
		return source, nil
	}
	// hclwrite needs a blank line after a one-line file with no trailing newline; otherwise a
	// newly-added attribute can be appended to the existing attribute's token stream.
	if len(source) > 0 && source[len(source)-1] != '\n' {
		source = append(append([]byte{}, source...), '\n')
	}
	file, diags := hclwrite.ParseConfig(source, filename, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, diags
	}
	body := file.Body()
	names := make([]string, 0, len(attrs))
	for name := range attrs {
		names = append(names, name)
	}
	sort.Strings(names)
	overlayName := fmt.Sprintf("%s flags for %s", fileType, filename)
	for _, name := range names {
		overlay, diags := hclwrite.ParseConfig(
			[]byte(fmt.Sprintf("%s = %s\n", name, attrs[name])), overlayName, hcl.Pos{Line: 1, Column: 1},
		)
		if diags.HasErrors() {
			return nil, fmt.Errorf("parse %s flag %s: %w", fileType, name, diags)
		}
		attr := overlay.Body().GetAttribute(name)
		if attr == nil {
			return nil, fmt.Errorf("parse %s flag %s: no attribute produced", fileType, name)
		}
		body.SetAttributeRaw(name, attr.Expr().BuildTokens(nil))
	}
	out := file.Bytes()
	return out, nil
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
		comment := flagUsage(input.Comment)

		switch typ {
		case schema.BoolType:
			flagFunc = func(name, extraHelp string) {
				flags.Bool(name, false, comment+extraHelp)
			}
		case schema.StringType, schema.IntType, schema.NumberType:
			flagFunc = func(name, extraHelp string) {
				flags.String(name, "", comment+extraHelp)
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

var langChoiceSpanRegexp = regexp.MustCompile(`(?s)<span\b[^>]*>(.*?)</span>`)

var envVarChoiceRegexp = regexp.MustCompile("(?s)(`[A-Z][A-Z0-9_]*`) or <span\\b[^>]*>.*?</span> environment variables")

func cleanComment(comment string) string {
	comment = envVarChoiceRegexp.ReplaceAllString(comment, "$1 environment variable")
	return langChoiceSpanRegexp.ReplaceAllString(comment, "$1")
}

func flagUsage(comment string) string {
	return strings.ReplaceAll(cleanComment(comment), "`", "")
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

var doDisplayStack = tokens.MustParseStackName("dev")

const doDisplayProject tokens.PackageName = "default"

func resourceURN(res *schema.Resource) resource.URN {
	_, _, name, diags := pcl.DecomposeToken(res.Token, hcl.Range{})
	contract.Assertf(!diags.HasErrors(), "token should decompose")
	return resource.NewURN(doDisplayStack.Q(), doDisplayProject, "", tokens.Type(res.Token), name)
}

func (pc *packageCommand) configureProvider(cmd *cobra.Command, ctx context.Context) error {
	// When --provider names an existing provider resource in the stack, start from that resource's
	// Inputs as the base; --provider-file and --input:* flags overlay on top so the user can
	// re-use a stack-stored provider's config and selectively override a property or two. The
	// stack-context check mirrors what we do for pulumi.organization / pulumi.stack in PCL
	// evaluation: it requires a project to be loaded and a stack to be selected in the workspace.
	// Snapshot the eval context once so the two reads here (the stack-context guard below and the
	// evaluateResourceFile call further down) see exactly the same view of the workspace.
	ec := pc.evalContext()
	var baseConfig resource.PropertyMap
	if pc.providerURN != "" {
		if ec.ProjectName == "" || ec.Stack == "" {
			return errors.New("--provider requires a stack context (a Pulumi project must be " +
				"present and a stack selected)")
		}
		base, err := pc.loadProviderInputsFromStack(ctx, resource.URN(pc.providerURN))
		if err != nil {
			return fmt.Errorf("--provider: %w", err)
		}
		baseConfig = base
	}

	config, err := evaluateResourceFile(
		ctx, pc.providerFile, "provider", pc.format,
		pc.providerDef, ec, pc.converter, pc.loaderTarget, pc.packageDescriptor,
		collectInputFlags(cmd, pc.spec.Name(), pc.providerDef.InputProperties),
	)
	if err != nil {
		return fmt.Errorf("parse provider file: %w", err)
	}

	// Merge: base from --provider gets overlaid by anything the user supplied via --provider-file
	// or --input:* flags. A property absent from the overlay falls through to the base.
	if baseConfig != nil {
		merged := maps.Clone(baseConfig)
		maps.Copy(merged, config)
		config = merged
	}

	urn := resource.NewURN("dev", "default", "", tokens.Type("pulumi:providers:"+pc.spec.Name()), "")
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

// loadProviderInputsFromStack opens the currently-selected stack via the workspace + login manager
// and returns the Inputs of the resource matching providerURN. Returns errors with context if no
// stack is selected, the stack can't be loaded, no resource matches the URN, or the matched
// resource isn't a provider — better to fail loudly than silently configure with junk.
func (pc *packageCommand) loadProviderInputsFromStack(
	ctx context.Context, providerURN resource.URN,
) (resource.PropertyMap, error) {
	s, err := cmdStack.RequireStack(
		ctx, pc.diagFwd, pc.ws, pc.lm,
		"", /*stackName — use whatever is currently selected*/
		cmdStack.LoadOnly, display.Options{Color: cmdutil.GetGlobalColorization()},
		"", /*configFile*/
	)
	if err != nil {
		return nil, fmt.Errorf("load stack: %w", err)
	}
	snap, err := s.Snapshot(ctx, backendSecrets.DefaultProvider)
	if err != nil {
		return nil, fmt.Errorf("load stack snapshot: %w", err)
	}
	if snap == nil {
		return nil, fmt.Errorf("stack has no snapshot yet; cannot resolve --provider %s", providerURN)
	}
	for _, res := range snap.Resources {
		if res.URN != providerURN {
			continue
		}
		// Sanity-check: the URN must refer to a provider resource. Providers have a type token of
		// the form "pulumi:providers:<pkg>"; anything else is almost certainly a user error.
		if !strings.HasPrefix(string(res.Type), "pulumi:providers:") {
			return nil, fmt.Errorf(
				"resource %s is not a provider (type=%s); --provider must name a provider resource",
				providerURN, res.Type,
			)
		}
		// The provider package must also match: AWS provider inputs handed to an Azure
		// Configure call would either fail with a confusing schema mismatch or — worse — silently
		// authenticate against the wrong cloud. Reject early with a clear message.
		expectedType := tokens.Type("pulumi:providers:" + pc.spec.Name())
		if res.Type != expectedType {
			return nil, fmt.Errorf(
				"resource %s is a provider for a different package (type=%s); --provider must name a %s resource",
				providerURN, res.Type, expectedType,
			)
		}
		// Clone so we don't hand callers an aliasing pointer into the snapshot's state.
		return maps.Clone(resource.ToResourcePropertyMap(res.Inputs)), nil
	}
	return nil, fmt.Errorf("no resource named %s in the current stack", providerURN)
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

// confirm prints summary and asks the user whether to proceed, using the same yes/no chooser as `pulumi up`
// and `pulumi destroy`. operation names the operation in the prompt (e.g. "create"). The summary and prompt
// go to stderr so that stdout stays a clean JSON channel for piping. Returns nil to proceed; a bail error
// (suppressed by the outer CLI) when the user declines; a real error when the prompt is cancelled (e.g.
// Ctrl-C), matching up/destroy. requireYesIfNonInteractive should have been called
// earlier; if we somehow reach here non-interactively without --yes we treat it as a decline.
func (pc *packageCommand) confirm(cmd *cobra.Command, summary, operation string, yes bool) error {
	stderr := cmd.ErrOrStderr()
	if summary != "" {
		fmt.Fprint(stderr, summary)
		if !strings.HasSuffix(summary, "\n") {
			fmt.Fprintln(stderr)
		}
	}
	if yes || pc.dryrun {
		return nil
	}
	if !cmdutil.Interactive() {
		return backenderr.ErrNonInteractiveRequiresYes
	}
	response, err := ui.PromptUserErr(
		fmt.Sprintf("Do you want to perform this %s?", operation),
		[]string{"yes", "no"},
		"no",
		cmdutil.GetGlobalColorization(),
		ui.SurveyStdio(cmd.InOrStdin(), stderr)...,
	)
	if err != nil {
		return fmt.Errorf("confirmation cancelled, not proceeding with the %s: %w", operation, err)
	}
	if response != "yes" {
		return result.FprintBailf(stderr, "confirmation declined")
	}
	return nil
}
