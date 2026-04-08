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

package runtime

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/ext/customdecode"
	"github.com/zclconf/go-cty/cty"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/util/pdag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

var invokeOptionsType = cty.ObjectWithOptionalAttrs(map[string]cty.Type{
	"version":   cty.String,
	"dependsOn": cty.List(cty.DynamicPseudoType),
	"provider":  cty.DynamicPseudoType,
}, []string{
	"version",
	"dependsOn",
	"provider",
})

type RunInfo struct {
	Project        string
	Stack          string
	Organization   string
	RootDirectory  string
	ProgramDir     string
	WorkingDir     string
	Config         map[string]string
	ConfigSecrets  []string
	MonitorAddress string
	EngineAddress  string
	LoaderAddress  string
	DryRun         bool
	Parallel       int32

	// PackageDescriptors are package blocks keyed by package name.
	PackageDescriptors map[string]*schema.PackageDescriptor
}

type Interpreter struct {
	program *pcl.Program
	info    RunInfo

	monitor pulumirpc.ResourceMonitorClient
	engine  pulumirpc.EngineClient
	loader  schema.ReferenceLoader

	// we write variables to the eval context in parallel during execution, so we need to synchronize access to it
	evalLock    sync.Mutex
	evalContext *hcl.EvalContext
	stackURN    string

	// namePrefix is prepended to resource and component names when this interpreter is executing
	// inside a component. For example, if a component named "myComp" contains a resource "res",
	// the resource is registered as "myComp-res". Nested components accumulate the prefix.
	namePrefix string

	// packageRefs are package references returned by RegisterPackage keyed by package name.
	packageRefs map[string]string
}

func NewInterpreter(program *pcl.Program, info RunInfo) *Interpreter {
	return &Interpreter{
		program:     program,
		info:        info,
		packageRefs: map[string]string{},
	}
}

// effectiveName returns the name to use when registering a resource or component with the given
// logical name, prepending the current namePrefix if one is set.
func (i *Interpreter) effectiveName(logicalName string) string {
	if i.namePrefix == "" {
		return logicalName
	}
	return i.namePrefix + "-" + logicalName
}

func (i *Interpreter) Run(ctx context.Context) error {
	if i.info.MonitorAddress == "" {
		return errors.New("missing monitor address")
	}

	monitorConn, err := grpc.NewClient(
		i.info.MonitorAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		rpcutil.GrpcChannelOptions(),
	)
	if err != nil {
		return fmt.Errorf("connect to monitor: %w", err)
	}
	defer contract.IgnoreClose(monitorConn)
	i.monitor = pulumirpc.NewResourceMonitorClient(monitorConn)

	loader, err := schema.NewLoaderClient(i.info.LoaderAddress)
	if err != nil {
		return fmt.Errorf("connect to loader: %w", err)
	}
	i.loader = schema.NewCachedLoader(loader)

	if i.info.EngineAddress != "" {
		engineConn, err := grpc.NewClient(
			i.info.EngineAddress,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			rpcutil.GrpcChannelOptions(),
		)
		if err != nil {
			return fmt.Errorf("connect to engine: %w", err)
		}
		defer contract.IgnoreClose(engineConn)
		i.engine = pulumirpc.NewEngineClient(engineConn)
	}

	i.evalContext = &hcl.EvalContext{
		Variables: map[string]cty.Value{},
		Functions: i.builtinFunctions(),
	}

	if diags := i.bindConfigVariables(ctx); diags.HasErrors() {
		return diags
	}

	if err := i.enforceRequiredVersion(ctx); err != nil {
		return err
	}

	if err := i.registerStack(ctx); err != nil {
		return err
	}

	if err := i.registerPackages(ctx); err != nil {
		return err
	}

	outputs, err := i.executeProgramNodes(ctx)
	if err != nil {
		return err
	}

	if err := i.registerStackOutputs(ctx, outputs); err != nil {
		return err
	}

	if i.monitor != nil {
		_, err := i.monitor.SignalAndWaitForShutdown(ctx, &emptypb.Empty{})
		if err != nil {
			if status, ok := status.FromError(err); ok && status.Code() == codes.Unimplemented {
				return nil
			}
			return err
		}
	}

	return nil
}

func (i *Interpreter) executeProgramNodes(ctx context.Context) (resource.PropertyMap, error) {
	dag := pdag.New[pcl.Node]()
	nodes := map[pcl.Node]pdag.Node{}

	for _, node := range i.program.Nodes {
		dagNode, done := dag.NewNode(node)
		done()
		nodes[node] = dagNode
	}

	for _, node := range i.program.Nodes {
		dagNodeA := nodes[node]
		for _, dep := range node.GetDependencies() {
			dagNodeB, ok := nodes[dep]
			contract.Assertf(ok, "missing node for dependency %s", dep.Name())
			err := dag.NewEdge(dagNodeB, dagNodeA)
			if err != nil {
				return nil, fmt.Errorf("failed to create edge from %s to %s: %w", dep.Name(), node.Name(), err)
			}
		}
	}

	var outputsLock sync.Mutex
	outputs := resource.PropertyMap{}
	err := dag.Walk(ctx, func(ctx context.Context, node pcl.Node) error {
		switch node := node.(type) {
		case *pcl.ConfigVariable:
			// handled before node execution
			return nil
		case *pcl.PulumiBlock:
			// handled before node execution
			return nil
		case *pcl.LocalVariable:
			value, poison, diags := i.evalExpression(node.Definition.Value)
			if poison != nil {
				i.setRawVariable(ctx, node.Name(), makePoisonValue(*poison))
				return nil
			}
			if diags.HasErrors() {
				return diags
			}
			if err := i.setVariable(ctx, node.Name(), value); err != nil {
				return fmt.Errorf("failed to set variable %s: %w", node.Name(), err)
			}
		case *pcl.Resource:
			if err := i.registerResource(ctx, node); err != nil {
				return fmt.Errorf("failed to register resource %s: %w", node.Name(), err)
			}
		case *pcl.Component:
			if err := i.registerComponent(ctx, node); err != nil {
				return fmt.Errorf("failed to register component %s: %w", node.Name(), err)
			}
		case *pcl.OutputVariable:
			value, poison, diags := i.evalExpression(node.Value)
			if poison != nil {
				return nil
			}
			if diags.HasErrors() {
				return diags
			}
			outputsLock.Lock()
			outputs[resource.PropertyKey(node.LogicalName())] = value
			outputsLock.Unlock()
		default:
			return fmt.Errorf("unknown node type: %T", node)
		}
		return nil
	}, pdag.MaxProcs(int(i.info.Parallel)))
	if err != nil {
		return nil, err
	}

	return outputs, nil
}

func (i *Interpreter) lookupResource(ctx context.Context, token string) (*schema.Resource, error) {
	pkg, mod, typ, diags := pcl.DecomposeToken(token, hcl.Range{})
	contract.Assertf(!diags.HasErrors(), "invalid token format for resource token %s", token)

	token = fmt.Sprintf("%s:%s:%s", pkg, mod, typ)
	pkgName := pkg
	if pkg == "pulumi" && mod == "providers" {
		pkgName = typ
	}

	descriptor := i.lookupPackageDescriptor(pkgName)
	pkgref, err := i.loader.LoadPackageReferenceV2(ctx, descriptor)
	if err != nil {
		return nil, fmt.Errorf("load package for token %s: %w", token, err)
	}
	if pkg == "pulumi" && mod == "providers" {
		return pkgref.Provider()
	}
	resources := pkgref.Resources()
	schemaResource, ok, err := resources.Get(token)
	if err != nil {
		return nil, fmt.Errorf("get resource from package for token %s: %w", token, err)
	}
	if !ok {
		// Didn't find the resource via a direct lookup, we now need to iterate _all_ the resources and use
		// TokenToModule to see if any of the match the token we have.
		iter := resources.Range()
		for iter.Next() {
			resToken := iter.Token()
			// Canonicalize the resources token via TokenToModule
			mod := pkgref.TokenToModule(resToken)
			if mod == "" {
				mod = "index"
			}
			pkg, _, typ, diags := pcl.DecomposeToken(resToken, hcl.Range{})
			contract.Assertf(!diags.HasErrors(), "invalid token format in package %s: %s", pkg, resToken)
			resToken = fmt.Sprintf("%s:%s:%s", pkg, mod, typ)
			if token == resToken {
				token = iter.Token()
				var err error
				schemaResource, err = iter.Resource()
				if err != nil {
					return nil, fmt.Errorf("get resource from package for token %s: %w", token, err)
				}
				break
			}
		}
	}
	if schemaResource == nil {
		return nil, fmt.Errorf("get resource from package for token %s", token)
	}
	return schemaResource, nil
}

func (i *Interpreter) lookupPackageDescriptor(pkgName string) *schema.PackageDescriptor {
	if descriptor, ok := i.info.PackageDescriptors[pkgName]; ok && descriptor != nil {
		return descriptor
	}
	return &schema.PackageDescriptor{Name: pkgName}
}

func PackageNameFromToken(token string) (string, error) {
	pkg, mod, name, diags := pcl.DecomposeToken(token, hcl.Range{})
	if diags.HasErrors() {
		return "", diags
	}
	if pkg == "pulumi" {
		if mod == "providers" {
			return name, nil
		}
		return "", nil
	}
	return pkg, nil
}

func (i *Interpreter) getPackageRefFromToken(token string) (string, error) {
	pkgName, err := PackageNameFromToken(token)
	if err != nil {
		return "", err
	}
	return i.packageRefs[pkgName], nil
}

func (i *Interpreter) registerPackages(ctx context.Context) error {
	if i.monitor == nil {
		return nil
	}

	keys := make([]string, 0, len(i.info.PackageDescriptors))
	for k := range i.info.PackageDescriptors {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		descriptor := i.info.PackageDescriptors[key]
		if descriptor == nil {
			continue
		}
		if descriptor.Parameterization == nil {
			continue
		}

		request := &pulumirpc.RegisterPackageRequest{
			Name:        descriptor.Name,
			DownloadUrl: descriptor.DownloadURL,
		}
		if descriptor.Version != nil {
			request.Version = descriptor.Version.String()
		}
		request.Parameterization = &pulumirpc.Parameterization{
			Name:    descriptor.Parameterization.Name,
			Version: descriptor.Parameterization.Version.String(),
			Value:   descriptor.Parameterization.Value,
		}

		resp, err := i.monitor.RegisterPackage(ctx, request)
		if err != nil {
			return fmt.Errorf("register package %q: %w", key, err)
		}
		if resp.GetRef() == "" {
			return fmt.Errorf("register package %q returned empty reference", key)
		}

		i.packageRefs[key] = resp.GetRef()
		i.packageRefs[descriptor.PackageName()] = resp.GetRef()
	}

	return nil
}

func (i *Interpreter) evalExpression(expr model.Expression) (resource.PropertyValue, *string, hcl.Diagnostics) {
	return i.evalExpressionWith(expr, i.evalContext)
}

// evalExpressionWith evaluates an expression using the given eval context (which may be a child of
// i.evalContext with additional variables, e.g. range.key/range.value for ranged resources).
func (i *Interpreter) evalExpressionWith(
	expr model.Expression, evalCtx *hcl.EvalContext,
) (resource.PropertyValue, *string, hcl.Diagnostics) {
	i.evalLock.Lock()
	value, diags := expr.Evaluate(evalCtx)
	i.evalLock.Unlock()
	if diags.HasErrors() {
		return resource.PropertyValue{}, nil, diags
	}
	pv, err := ctyToPropertyValue(value)
	if err != nil {
		var poison *poisonError
		if errors.As(err, &poison) {
			return resource.PropertyValue{}, &poison.name, nil
		}
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  err.Error(),
		})
		return resource.PropertyValue{}, nil, diags
	}
	return pv, nil, diags
}

func (i *Interpreter) bindConfigVariables(ctx context.Context) hcl.Diagnostics {
	var diagnostics hcl.Diagnostics
	secretKeys := map[string]struct{}{}
	for _, key := range i.info.ConfigSecrets {
		secretKeys[key] = struct{}{}
	}
	for _, cfg := range i.program.ConfigVariables() {
		key := fmt.Sprintf("%s:%s", i.info.Project, cfg.LogicalName())
		raw, has := i.info.Config[key]
		if !has {
			if cfg.DefaultValue != nil {
				value, poison, diags := i.evalExpression(cfg.DefaultValue)
				contract.Assertf(poison == nil, "config variables can't be poisoned")
				diagnostics = append(diagnostics, diags...)
				if _, isSecret := secretKeys[key]; isSecret || cfg.Secret {
					value = resource.MakeSecret(value)
				}
				if !diags.HasErrors() {
					if err := i.setVariable(ctx, cfg.Name(), value); err != nil {
						diagnostics = append(diagnostics, &hcl.Diagnostic{
							Severity: hcl.DiagError,
							Summary:  err.Error(),
						})
					}
				}
				continue
			}
			if !cfg.Nullable {
				rng := cfg.SyntaxNode().Range()
				diagnostics = append(diagnostics, &hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  fmt.Sprintf("missing required configuration value %q", cfg.LogicalName()),
					Subject:  &rng,
				})
			}
			continue
		}

		value, diags := parseConfigPropertyValue(raw, cfg.Type())
		diagnostics = append(diagnostics, diags...)
		if !diags.HasErrors() {
			if _, isSecret := secretKeys[key]; isSecret || cfg.Secret {
				value = resource.MakeSecret(value)
			}
			if err := i.setVariable(ctx, cfg.Name(), value); err != nil {
				diagnostics = append(diagnostics, &hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  err.Error(),
				})
			}
		}
	}
	return diagnostics
}

func (i *Interpreter) enforceRequiredVersion(ctx context.Context) error {
	var required model.Expression
	for _, node := range i.program.Nodes {
		if block, ok := node.(*pcl.PulumiBlock); ok {
			required = block.RequiredVersion
			break
		}
	}
	if required == nil {
		return nil
	}

	value, poison, diags := i.evalExpression(required)
	if poison != nil {
		return fmt.Errorf("could not evaluate requiredVersion because of failure from %s", *poison)
	}
	if diags.HasErrors() {
		return diags
	}
	if !value.IsString() {
		return errors.New("requiredVersion must be a string")
	}
	if i.engine == nil {
		return errors.New("engine client not available to validate requiredVersion")
	}

	_, err := i.engine.RequirePulumiVersion(ctx, &pulumirpc.RequirePulumiVersionRequest{
		PulumiVersionRange: value.StringValue(),
	})
	return err
}

func (i *Interpreter) registerStack(ctx context.Context) error {
	stackName := fmt.Sprintf("%s-%s", i.info.Project, i.info.Stack)
	resp, err := i.monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
		Type:   string(resource.RootStackType),
		Name:   stackName,
		Custom: false,
	})
	if err != nil {
		return err
	}
	if resp.GetResult() != pulumirpc.Result_SUCCESS {
		return fmt.Errorf("stack registration failed: %s", resp.GetResult())
	}
	if resp.GetUrn() == "" {
		return errors.New("stack URN was empty")
	}
	if resp.Object != nil {
		marshalOpts := plugin.MarshalOptions{
			KeepUnknowns:  true,
			KeepSecrets:   true,
			KeepResources: true,
		}
		objectValue, err := plugin.UnmarshalProperties(resp.Object, marshalOpts)
		if err != nil {
			return fmt.Errorf("unmarshal stack outputs: %w", err)
		}

		err = i.setVariable(ctx, "pulumi", resource.NewProperty(objectValue))
		if err != nil {
			return fmt.Errorf("set pulumi variable: %w", err)
		}
	}
	i.stackURN = resp.GetUrn()
	return nil
}

func getAllDependencies(value resource.PropertyValue) []string {
	if value.IsOutput() {
		output := value.OutputValue()
		deps := output.Dependencies
		strDeps := make([]string, len(deps))
		for i, dep := range deps {
			strDeps[i] = string(dep)
		}
		return append(strDeps, getAllDependencies(output.Element)...)
	}
	if value.IsObject() {
		var deps []string
		for _, v := range value.ObjectValue() {
			deps = append(deps, getAllDependencies(v)...)
		}
		return deps
	}
	if value.IsArray() {
		var deps []string
		for _, v := range value.ArrayValue() {
			deps = append(deps, getAllDependencies(v)...)
		}
		return deps
	}
	return nil
}

func unwrapOutputs(value resource.PropertyValue) (resource.PropertyValue, []resource.URN) {
	if value.IsOutput() {
		o := value.OutputValue()
		val, deps := unwrapOutputs(o.Element)
		if o.Secret {
			val = resource.MakeSecret(val)
		}
		return val, append(o.Dependencies, deps...)
	}
	if value.IsSecret() {
		val, deps := unwrapOutputs(value.SecretValue().Element)
		return resource.MakeSecret(val), deps
	}
	if value.IsArray() {
		var arr []resource.PropertyValue
		var deps []resource.URN
		for _, v := range value.ArrayValue() {
			val, d := unwrapOutputs(v)
			arr = append(arr, val)
			deps = append(deps, d...)
		}
		return resource.NewProperty(arr), deps
	}
	if value.IsObject() {
		obj := resource.PropertyMap{}
		var deps []resource.URN
		for k, v := range value.ObjectValue() {
			val, d := unwrapOutputs(v)
			obj[k] = val
			deps = append(deps, d...)
		}
		return resource.NewProperty(obj), deps
	}
	return value, nil
}

func unwrapResource(value resource.PropertyValue) (string, resource.PropertyValue, error) {
	value, _ = unwrapOutputs(value)
	if !value.IsObject() {
		return "", resource.PropertyValue{}, fmt.Errorf("expected resource object, got %s", value.TypeString())
	}
	obj := value.ObjectValue()
	urnVal, ok := obj["urn"]
	if !ok || urnVal.IsNull() || urnVal.IsComputed() || !urnVal.IsString() {
		return "", resource.PropertyValue{}, errors.New("expected resource object with known urn property")
	}
	idVal, ok := obj["id"]
	if !ok || idVal.IsNull() {
		return "", resource.PropertyValue{}, errors.New("expected resource object with id property of type string")
	}

	if !idVal.IsComputed() && !idVal.IsString() {
		return "", resource.PropertyValue{}, errors.New("expected resource object with id property of type string")
	}

	return urnVal.StringValue(), idVal, nil
}

func collapseResourceReferences(value resource.PropertyValue) resource.PropertyValue {
	switch {
	case value.IsOutput():
		output := value.OutputValue()
		newOutput := resource.Output{
			Dependencies: output.Dependencies,
			Secret:       output.Secret,
			Known:        output.Known,
			Element:      collapseResourceReferences(output.Element),
		}
		// If this is an output for a single URN and that URN is now the inner resource reference value then we can
		// collapse this output into a resource reference directly.
		if len(newOutput.Dependencies) == 1 &&
			output.Known &&
			!output.Element.IsResourceReference() &&
			newOutput.Element.IsResourceReference() &&
			newOutput.Element.ResourceReferenceValue().URN == newOutput.Dependencies[0] {
			if newOutput.Secret {
				return resource.MakeSecret(newOutput.Element)
			}
			return newOutput.Element
		}
		return resource.NewProperty(newOutput)
	case value.IsSecret():
		secret := value.SecretValue()
		secret.Element = collapseResourceReferences(secret.Element)
		return resource.NewProperty(secret)
	case value.IsArray():
		array := value.ArrayValue()
		collapsed := make([]resource.PropertyValue, len(array))
		for i, elem := range array {
			collapsed[i] = collapseResourceReferences(elem)
		}
		return resource.NewProperty(collapsed)
	case value.IsObject():
		obj := value.ObjectValue()
		collapsed := make(resource.PropertyMap, len(obj))
		for key, elem := range obj {
			collapsed[key] = collapseResourceReferences(elem)
		}

		// Resource references are expanded into resource-shaped objects for evaluation.
		// Collapse these back before marshaling register requests.
		urn, hasURN := collapsed["urn"]
		id, hasID := collapsed["id"]
		typ, hasType := collapsed["__type"]
		if hasURN && hasID && hasType && urn.IsString() && typ.IsString() {
			return resource.NewProperty(resource.ResourceReference{
				URN: resource.URN(urn.StringValue()),
				ID:  id,
			})
		}
		return resource.NewProperty(collapsed)
	default:
		return value
	}
}

func (i *Interpreter) registerResource(ctx context.Context, res *pcl.Resource) error {
	lexicalBaseName := res.Name()
	logicalBaseName := i.effectiveName(res.LogicalName())
	if res.Options == nil || res.Options.Range == nil {
		result, err := i.registerResourceWith(ctx, res, i.evalContext, logicalBaseName)
		if err != nil {
			return err
		}
		i.setRawVariable(ctx, lexicalBaseName, result)
		return nil
	}

	rangeValue, poison, diags := i.evalExpression(res.Options.Range)
	if poison != nil {
		i.setRawVariable(ctx, lexicalBaseName, makePoisonValue(*poison))
	}
	if diags.HasErrors() {
		return diags
	}

	// Unwrap any output values, but if the range is computed we just have to skip this resource
	rangeValue, _ = unwrapOutputs(rangeValue)
	if rangeValue.IsComputed() {
		return nil
	}

	if rangeValue.IsBool() {
		if !rangeValue.BoolValue() {
			return nil
		}
		result, err := i.registerResourceWith(ctx, res, i.evalContext, logicalBaseName)
		if err != nil {
			return err
		}
		i.setRawVariable(ctx, lexicalBaseName, result)
		return nil
	}

	makeRangeCtx := func(key cty.Value, value cty.Value) *hcl.EvalContext {
		rangeCtx := i.evalContext.NewChild()
		rangeVars := map[string]cty.Value{"value": value}
		if key != cty.NilVal {
			rangeVars["key"] = key
		}
		rangeCtx.Variables = map[string]cty.Value{
			"range": cty.ObjectVal(rangeVars),
		}
		return rangeCtx
	}

	registerMany := func(items []struct {
		suffix  string
		evalCtx *hcl.EvalContext
	},
	) error {
		results := make([]cty.Value, 0, len(items))
		for _, item := range items {
			name := fmt.Sprintf("%s-%s", logicalBaseName, item.suffix)
			result, err := i.registerResourceWith(ctx, res, item.evalCtx, name)
			if err != nil {
				return err
			}
			results = append(results, result)
		}
		if len(results) == 0 {
			i.setRawVariable(ctx, lexicalBaseName, cty.ListValEmpty(cty.DynamicPseudoType))
			return nil
		}
		i.setRawVariable(ctx, lexicalBaseName, cty.ListVal(results))
		return nil
	}

	if rangeValue.IsNumber() {
		count := int(rangeValue.NumberValue())
		if count < 0 {
			count = 0
		}
		items := make([]struct {
			suffix  string
			evalCtx *hcl.EvalContext
		}, 0, count)
		for idx := 0; idx < count; idx++ {
			idxVal := cty.NumberIntVal(int64(idx))
			items = append(items, struct {
				suffix  string
				evalCtx *hcl.EvalContext
			}{
				suffix:  strconv.Itoa(idx),
				evalCtx: makeRangeCtx(cty.NilVal, idxVal),
			})
		}
		return registerMany(items)
	}

	if rangeValue.IsArray() {
		values := rangeValue.ArrayValue()
		items := make([]struct {
			suffix  string
			evalCtx *hcl.EvalContext
		}, 0, len(values))
		for idx, v := range values {
			val, err := propertyValueToCty(ctx, i.monitor, v)
			if err != nil {
				return err
			}
			items = append(items, struct {
				suffix  string
				evalCtx *hcl.EvalContext
			}{
				suffix:  strconv.Itoa(idx),
				evalCtx: makeRangeCtx(cty.NumberIntVal(int64(idx)), val),
			})
		}
		return registerMany(items)
	}

	if rangeValue.IsObject() {
		values := rangeValue.ObjectValue()
		keys := make([]string, 0, len(values))
		for k := range values {
			keys = append(keys, string(k))
		}
		sort.Strings(keys)
		resultMap := make(map[string]cty.Value, len(keys))
		for _, key := range keys {
			val, err := propertyValueToCty(ctx, i.monitor, values[resource.PropertyKey(key)])
			if err != nil {
				return err
			}
			name := fmt.Sprintf("%s-%s", logicalBaseName, key)
			result, err := i.registerResourceWith(ctx, res, makeRangeCtx(cty.StringVal(key), val), name)
			if err != nil {
				return err
			}
			resultMap[key] = result
		}
		if len(resultMap) == 0 {
			i.setRawVariable(ctx, lexicalBaseName, cty.EmptyObjectVal)
		} else {
			i.setRawVariable(ctx, lexicalBaseName, cty.ObjectVal(resultMap))
		}
		return nil
	}

	return fmt.Errorf("unsupported range type for resource %s", res.Name())
}

func (i *Interpreter) registerResourceWith(
	ctx context.Context, res *pcl.Resource, evalCtx *hcl.EvalContext, logicalName string,
) (cty.Value, error) {
	schemaResource, err := i.lookupResource(ctx, res.Token)
	if err != nil {
		return cty.NilVal, fmt.Errorf("lookup resource schema for token %s: %w", res.Token, err)
	}
	token := res.Token
	if schemaResource != nil {
		token = schemaResource.Token
	}

	inputPropertyTypes := map[string]schema.Type{}
	if schemaResource != nil {
		for _, property := range schemaResource.InputProperties {
			inputPropertyTypes[property.Name] = property.Type
		}
	}

	inputs := resource.PropertyMap{}
	for _, attr := range res.Inputs {
		targetType := attr.Value.Type()
		if obj, ok := res.InputType.(*model.ObjectType); ok {
			if prop, ok := obj.Properties[attr.Name]; ok {
				targetType = prop
			}
		}

		expr, diags := pcl.RewriteConversions(attr.Value, targetType)
		if diags.HasErrors() {
			return cty.NilVal, diags
		}

		val, poison, diags := i.evalExpressionWith(expr, evalCtx)
		if poison != nil {
			return makePoisonValue(*poison), nil
		}
		if diags.HasErrors() {
			return cty.NilVal, diags
		}
		propertyValue := collapseResourceReferences(val)
		if inputPropertyType, ok := inputPropertyTypes[attr.Name]; ok {
			converted, err := convertPropertyValueForSchemaType(propertyValue, inputPropertyType)
			if err != nil {
				return cty.NilVal, err
			}
			propertyValue = converted
		}
		inputs[resource.PropertyKey(attr.Name)] = propertyValue
	}

	if schemaResource != nil {
		applySchemaInputDefaults(inputs, schemaResource)
		for _, input := range schemaResource.InputProperties {
			if input.Secret {
				key := resource.PropertyKey(input.Name)
				attr, ok := inputs[key]
				if ok && !attr.IsSecret() {
					inputs[key] = resource.MakeSecret(attr)
				}
			}
		}
	}

	marshalOpts := plugin.MarshalOptions{
		KeepUnknowns:  true,
		KeepSecrets:   true,
		KeepResources: true,
	}
	obj, err := plugin.MarshalProperties(inputs, marshalOpts)
	if err != nil {
		return cty.NilVal, err
	}
	// The rest of this method can send output values
	marshalOpts.KeepOutputValues = true

	custom := true
	if schemaResource != nil {
		custom = !schemaResource.IsComponent
	}

	dependencies := []string{}
	propertyDependencies := map[string]*pulumirpc.RegisterResourceRequest_PropertyDependencies{}
	for key, val := range inputs {
		deps := getAllDependencies(val)
		if len(deps) > 0 {
			dependencies = append(dependencies, deps...)
			propertyDependencies[string(key)] = &pulumirpc.RegisterResourceRequest_PropertyDependencies{
				Urns: deps,
			}
		}
	}

	request := &pulumirpc.RegisterResourceRequest{
		Type:                    token,
		Name:                    logicalName,
		Custom:                  custom,
		Remote:                  !custom,
		Object:                  obj,
		PropertyDependencies:    propertyDependencies,
		Dependencies:            dependencies,
		AcceptSecrets:           true,
		AcceptResources:         true,
		SupportsResultReporting: true,
	}
	packageRef, err := i.getPackageRefFromToken(token)
	if err != nil {
		return cty.NilVal, err
	}
	request.PackageRef = packageRef

	if res.Options != nil {
		if res.Options.AdditionalSecretOutputs != nil {
			additionalSecretOutputs, poison, diags := i.evalExpressionWith(res.Options.AdditionalSecretOutputs, evalCtx)
			if poison != nil {
				return makePoisonValue(*poison), nil
			}
			if diags.HasErrors() {
				return cty.NilVal, diags
			}
			if !additionalSecretOutputs.IsNull() && !additionalSecretOutputs.IsComputed() {
				if !additionalSecretOutputs.IsArray() {
					return cty.NilVal, errors.New("additionalSecretOutputs must be an array of strings")
				}
				var additionalSecretOutputKeys []string
				for _, v := range additionalSecretOutputs.ArrayValue() {
					if v.IsNull() || v.IsComputed() {
						continue
					}
					if !v.IsString() {
						return cty.NilVal, errors.New("additionalSecretOutputs must be an array of strings")
					}
					additionalSecretOutputKeys = append(additionalSecretOutputKeys, v.StringValue())
				}
				request.AdditionalSecretOutputs = additionalSecretOutputKeys
			}
		}
		if res.Options.Aliases != nil {
			aliases, poison, diags := i.evalExpressionWith(res.Options.Aliases, evalCtx)
			if poison != nil {
				return makePoisonValue(*poison), nil
			}
			if diags.HasErrors() {
				return cty.NilVal, diags
			}
			if !aliases.IsNull() && !aliases.IsComputed() {
				if !aliases.IsArray() {
					return cty.NilVal, errors.New("aliases must be an array of strings or alias objects")
				}
				var aliasOpts []*pulumirpc.Alias
				// Translate each alias expression (either string or object) into an rpc alias object
				for _, alias := range aliases.ArrayValue() {
					if alias.IsString() {
						aliasOpts = append(aliasOpts, &pulumirpc.Alias{
							Alias: &pulumirpc.Alias_Urn{
								Urn: alias.StringValue(),
							},
						})
					} else if alias.IsObject() {
						obj := alias.ObjectValue()
						aliasOpt := &pulumirpc.Alias_Spec{}

						setString := func(field resource.PropertyKey, setter func(string)) error {
							attr, ok := obj[field]
							if ok && !attr.IsNull() && !attr.IsComputed() {
								if !attr.IsString() {
									return fmt.Errorf("%s must be a string", field)
								}
								setter(attr.StringValue())
							}
							return nil
						}

						err := setString("name", func(name string) { aliasOpt.Name = name })
						if err != nil {
							return cty.NilVal, err
						}
						err = setString("type", func(typ string) { aliasOpt.Type = typ })
						if err != nil {
							return cty.NilVal, err
						}

						noParent, ok := obj["noParent"]
						if ok && !noParent.IsNull() && !noParent.IsComputed() {
							if !noParent.IsBool() {
								return cty.NilVal, errors.New("noParent must be a boolean")
							}
							aliasOpt.Parent = &pulumirpc.Alias_Spec_NoParent{
								NoParent: noParent.BoolValue(),
							}
						}

						parent, ok := obj["parent"]
						if ok && !parent.IsNull() && !parent.IsComputed() {
							urn, _, err := unwrapResource(parent)
							if err != nil {
								return cty.NilVal, fmt.Errorf("parent: %w", err)
							}
							aliasOpt.Parent = &pulumirpc.Alias_Spec_ParentUrn{
								ParentUrn: urn,
							}
						}

						aliasOpts = append(aliasOpts, &pulumirpc.Alias{
							Alias: &pulumirpc.Alias_Spec_{
								Spec: aliasOpt,
							},
						})
					} else {
						return cty.NilVal, errors.New("aliases must be an array of strings or alias objects")
					}
				}
				request.Aliases = aliasOpts
			}
		}
		if res.Options.DependsOn != nil {
			dependsOn, poison, diags := i.evalExpressionWith(res.Options.DependsOn, evalCtx)
			if poison != nil {
				return makePoisonValue(*poison), nil
			}
			if diags.HasErrors() {
				return cty.NilVal, diags
			}
			if !dependsOn.IsNull() && !dependsOn.IsComputed() {
				if !dependsOn.IsArray() {
					return cty.NilVal, errors.New("dependsOn must be an array of resource objects")
				}
				var dependsOnUrns []string
				for _, v := range dependsOn.ArrayValue() {
					if v.IsNull() || v.IsComputed() {
						continue
					}
					urn, _, err := unwrapResource(v)
					if err != nil {
						return cty.NilVal, fmt.Errorf("dependsOn: %w", err)
					}
					dependsOnUrns = append(dependsOnUrns, urn)
				}
				request.Dependencies = append(request.Dependencies, dependsOnUrns...)
			}
		}
		if res.Options.EnvVarMappings != nil {
			envVars, poison, diags := i.evalExpressionWith(res.Options.EnvVarMappings, evalCtx)
			if poison != nil {
				return makePoisonValue(*poison), nil
			}
			if diags.HasErrors() {
				return cty.NilVal, diags
			}
			if !envVars.IsNull() && !envVars.IsComputed() {
				if !envVars.IsObject() {
					return cty.NilVal, errors.New(
						"envVarMappings must be an object mapping environment variable names to input property keys")
				}
				envVarMappings := map[string]string{}
				for envVar, propKey := range envVars.ObjectValue() {
					if propKey.IsNull() || propKey.IsComputed() || !propKey.IsString() {
						return cty.NilVal, errors.New(
							"envVarMappings must be an object mapping environment variable names to input property keys")
					}
					envVarMappings[string(envVar)] = propKey.StringValue()
				}
				request.EnvVarMappings = envVarMappings
			}
		}
		if res.Options.ImportID != nil {
			importID, poison, diags := i.evalExpressionWith(res.Options.ImportID, evalCtx)
			if poison != nil {
				return makePoisonValue(*poison), nil
			}
			if diags.HasErrors() {
				return cty.NilVal, diags
			}
			if !importID.IsNull() && !importID.IsComputed() {
				if !importID.IsString() {
					return cty.NilVal, errors.New("import must be a string")
				}
				request.ImportId = importID.StringValue()
			}
		}
		if res.Options.IgnoreChanges != nil {
			ignoreChanges, poison, diags := i.evalExpressionWith(res.Options.IgnoreChanges, evalCtx)
			if poison != nil {
				return makePoisonValue(*poison), nil
			}
			if diags.HasErrors() {
				return cty.NilVal, diags
			}
			if !ignoreChanges.IsNull() && !ignoreChanges.IsComputed() {
				if !ignoreChanges.IsArray() {
					return cty.NilVal, errors.New("ignoreChanges must be an array of strings")
				}
				icopt := []string{}
				for _, v := range ignoreChanges.ArrayValue() {
					if v.IsNull() || v.IsComputed() {
						continue
					}
					if !v.IsString() {
						return cty.NilVal, errors.New("ignoreChanges must be an array of strings")
					}
					icopt = append(icopt, v.StringValue())
				}
				request.IgnoreChanges = icopt
			}
		}
		if res.Options.Protect != nil {
			protect, poison, diags := i.evalExpressionWith(res.Options.Protect, evalCtx)
			if poison != nil {
				return makePoisonValue(*poison), nil
			}
			if diags.HasErrors() {
				return cty.NilVal, diags
			}
			if !protect.IsComputed() {
				var popt *bool
				if protect.IsBool() {
					b := protect.BoolValue()
					popt = &b
				} else if !protect.IsNull() {
					return cty.NilVal, errors.New("protect must be a boolean or null")
				}
				request.Protect = popt
			}
		}
		if res.Options.ReplaceWith != nil {
			replaceWith, poison, diags := i.evalExpressionWith(res.Options.ReplaceWith, evalCtx)
			if poison != nil {
				return makePoisonValue(*poison), nil
			}
			if diags.HasErrors() {
				return cty.NilVal, diags
			}
			if !replaceWith.IsNull() && !replaceWith.IsComputed() {
				if !replaceWith.IsArray() {
					return cty.NilVal, errors.New("replaceWith must be an array of resources")
				}
				var rwopt []string
				for _, v := range replaceWith.ArrayValue() {
					if v.IsNull() || v.IsComputed() {
						continue
					}
					urn, _, err := unwrapResource(v)
					if err != nil {
						return cty.NilVal, fmt.Errorf("replaceWith: %w", err)
					}
					rwopt = append(rwopt, urn)
				}
				request.ReplaceWith = rwopt
			}
		}
		if res.Options.ReplaceOnChanges != nil {
			replaceOnChanges, poison, diags := i.evalExpressionWith(res.Options.ReplaceOnChanges, evalCtx)
			if poison != nil {
				return makePoisonValue(*poison), nil
			}
			if diags.HasErrors() {
				return cty.NilVal, diags
			}
			if !replaceOnChanges.IsNull() && !replaceOnChanges.IsComputed() {
				if !replaceOnChanges.IsArray() {
					return cty.NilVal, errors.New("replaceOnChanges must be an array of strings")
				}
				rocopt := []string{}
				for _, v := range replaceOnChanges.ArrayValue() {
					if v.IsNull() || v.IsComputed() {
						continue
					}
					if !v.IsString() {
						return cty.NilVal, errors.New("replaceOnChanges must be an array of strings")
					}
					rocopt = append(rocopt, v.StringValue())
				}
				request.ReplaceOnChanges = rocopt
			}
		}
		if res.Options.ReplacementTrigger != nil {
			replacement, poison, diags := i.evalExpressionWith(res.Options.ReplacementTrigger, evalCtx)
			if poison != nil {
				return makePoisonValue(*poison), nil
			}
			if diags.HasErrors() {
				return cty.NilVal, diags
			}
			request.ReplacementTrigger, err = plugin.MarshalPropertyValue(
				"replacementTrigger", replacement, marshalOpts)
			if err != nil {
				return cty.NilVal, err
			}
		}
		if res.Options.RetainOnDelete != nil {
			retain, poison, diags := i.evalExpressionWith(res.Options.RetainOnDelete, evalCtx)
			if poison != nil {
				return makePoisonValue(*poison), nil
			}
			if diags.HasErrors() {
				return cty.NilVal, diags
			}
			if !retain.IsNull() && !retain.IsComputed() {
				var retainOnDelete *bool
				if retain.IsBool() {
					b := retain.BoolValue()
					retainOnDelete = &b
				} else {
					return cty.NilVal, errors.New("retainOnDelete must be a boolean or null")
				}
				request.RetainOnDelete = retainOnDelete
			}
		}
		if res.Options.Version != nil {
			version, poison, diags := i.evalExpressionWith(res.Options.Version, evalCtx)
			if poison != nil {
				return makePoisonValue(*poison), nil
			}
			if diags.HasErrors() {
				return cty.NilVal, diags
			}
			if !version.IsNull() && !version.IsComputed() {
				if !version.IsString() {
					return cty.NilVal, errors.New("version must be a string")
				}
				request.Version = version.StringValue()
			}
		}
		if res.Options.CustomTimeouts != nil {
			timeouts, poison, diags := i.evalExpressionWith(res.Options.CustomTimeouts, evalCtx)
			if poison != nil {
				return makePoisonValue(*poison), nil
			}
			if diags.HasErrors() {
				return cty.NilVal, diags
			}
			if !timeouts.IsNull() && !timeouts.IsComputed() {
				if !timeouts.IsObject() {
					return cty.NilVal, errors.New("customTimeouts must be an object")
				}
				timeoutValues := map[string]string{}
				for k, v := range timeouts.ObjectValue() {
					if v.IsNull() || v.IsComputed() {
						continue
					}
					if !v.IsString() {
						return cty.NilVal, fmt.Errorf("customTimeouts.%s must be a string", k)
					}
					timeoutValues[string(k)] = v.StringValue()
				}
				request.CustomTimeouts = &pulumirpc.RegisterResourceRequest_CustomTimeouts{
					Create: timeoutValues["create"],
					Update: timeoutValues["update"],
					Delete: timeoutValues["delete"],
				}
			}
		}
		if res.Options.DeleteBeforeReplace != nil {
			dbr, poison, diags := i.evalExpressionWith(res.Options.DeleteBeforeReplace, evalCtx)
			if poison != nil {
				return makePoisonValue(*poison), nil
			}
			if diags.HasErrors() {
				return cty.NilVal, diags
			}
			if !dbr.IsNull() && !dbr.IsComputed() {
				if dbr.IsBool() {
					request.DeleteBeforeReplace = dbr.BoolValue()
					request.DeleteBeforeReplaceDefined = true
				} else if !dbr.IsNull() {
					return cty.NilVal, errors.New("deleteBeforeReplace must be a boolean or null")
				}
			}
		}
		if res.Options.DeletedWith != nil {
			deletedWith, poison, diags := i.evalExpressionWith(res.Options.DeletedWith, evalCtx)
			if poison != nil {
				return makePoisonValue(*poison), nil
			}
			if diags.HasErrors() {
				return cty.NilVal, diags
			}
			if !deletedWith.IsNull() && !deletedWith.IsComputed() {
				urn, _, err := unwrapResource(deletedWith)
				if err != nil {
					return cty.NilVal, fmt.Errorf("deletedWith: %w", err)
				}
				request.DeletedWith = urn
			}
		}
		if res.Options.PluginDownloadURL != nil {
			downloadURL, poison, diags := i.evalExpressionWith(res.Options.PluginDownloadURL, evalCtx)
			if poison != nil {
				return makePoisonValue(*poison), nil
			}
			if diags.HasErrors() {
				return cty.NilVal, diags
			}
			if !downloadURL.IsNull() && !downloadURL.IsComputed() {
				if !downloadURL.IsString() {
					return cty.NilVal, errors.New("pluginDownloadURL must be a string")
				}
				request.PluginDownloadURL = downloadURL.StringValue()
			}
		}
		if res.Options.Parent != nil {
			parent, poison, diags := i.evalExpressionWith(res.Options.Parent, evalCtx)
			if poison != nil {
				return makePoisonValue(*poison), nil
			}
			if diags.HasErrors() {
				return cty.NilVal, diags
			}
			if !parent.IsNull() && !parent.IsComputed() {
				urn, _, err := unwrapResource(parent)
				if err != nil {
					return cty.NilVal, fmt.Errorf("parent: %w", err)
				}
				request.Parent = urn
			}
		}
		if res.Options.Provider != nil {
			provider, poison, diags := i.evalExpressionWith(res.Options.Provider, evalCtx)
			if poison != nil {
				return makePoisonValue(*poison), nil
			}
			if diags.HasErrors() {
				return cty.NilVal, diags
			}
			if !provider.IsNull() && !provider.IsComputed() {
				urn, id, err := unwrapResource(provider)
				if err != nil {
					return cty.NilVal, fmt.Errorf("provider: %w", err)
				}
				var idstr string
				if id.IsString() {
					idstr = id.StringValue()
				} else {
					idstr = plugin.UnknownStringValue
				}
				request.Provider = fmt.Sprintf("%s::%s", urn, idstr)
			}
		}
		if res.Options.Providers != nil {
			providers, poison, diags := i.evalExpressionWith(res.Options.Providers, evalCtx)
			if poison != nil {
				return makePoisonValue(*poison), nil
			}
			if diags.HasErrors() {
				return cty.NilVal, diags
			}
			if !providers.IsNull() && !providers.IsComputed() {
				// Providers is either a list of provider objects or a map of provider name to provider objects. We need
				// to support both forms and translate the list into the map form expected by the RPC.
				psopt := map[string]string{}
				if providers.IsObject() {
					for k, v := range providers.ObjectValue() {
						urn, id, err := unwrapResource(v)
						if err != nil {
							return cty.NilVal, fmt.Errorf("providers: %w", err)
						}
						var idstr string
						if id.IsString() {
							idstr = id.StringValue()
						} else {
							idstr = plugin.UnknownStringValue
						}
						psopt[string(k)] = fmt.Sprintf("%s::%s", urn, idstr)
					}
				} else if providers.IsArray() {
					for _, v := range providers.ArrayValue() {
						urn, id, err := unwrapResource(v)
						if err != nil {
							return cty.NilVal, fmt.Errorf("providers: %w", err)
						}
						typ := resource.URN(urn).Type()
						_, _, pkg, diags := pcl.DecomposeToken(string(typ), hcl.Range{})
						contract.Assertf(!diags.HasErrors(), "invalid token format from URN %s: %s", urn, typ)

						var idstr string
						if id.IsString() {
							idstr = id.StringValue()
						} else {
							idstr = plugin.UnknownStringValue
						}
						psopt[pkg] = fmt.Sprintf("%s::%s", urn, idstr)
					}
				} else {
					return cty.NilVal, errors.New(
						"providers must be an array of provider objects or a map of provider name to provider objects")
				}
				request.Providers = psopt
			}
		}
		if res.Options.HideDiffs != nil {
			hideDiffs, poison, diags := i.evalExpressionWith(res.Options.HideDiffs, evalCtx)
			if poison != nil {
				return makePoisonValue(*poison), nil
			}
			if diags.HasErrors() {
				return cty.NilVal, diags
			}
			if !hideDiffs.IsNull() && !hideDiffs.IsComputed() {
				if !hideDiffs.IsArray() {
					return cty.NilVal, errors.New("hideDiffs must be an array of strings")
				}
				hdopt := []string{}
				for _, v := range hideDiffs.ArrayValue() {
					if v.IsNull() || v.IsComputed() {
						continue
					}
					if !v.IsString() {
						return cty.NilVal, errors.New("hideDiffs must be an array of strings")
					}
					hdopt = append(hdopt, v.StringValue())
				}
				request.HideDiffs = hdopt
			}
		}
	}

	// Add schema-based replaceOnChanges paths.
	if schemaResource != nil {
		schemaReplaceOnChanges, _ := schemaResource.ReplaceOnChanges()
		request.ReplaceOnChanges = append(
			request.ReplaceOnChanges,
			schema.PropertyListJoinToString(schemaReplaceOnChanges, func(s string) string { return s })...)
	}

	// Default parent to the stack if not specified
	if request.Parent == "" {
		request.Parent = i.stackURN
	}

	resp, err := i.monitor.RegisterResource(ctx, request)
	if err != nil {
		return cty.NilVal, err
	}
	if resp.GetResult() != pulumirpc.Result_SUCCESS {
		// This resource failed to register but we might be running with --continue-on-error so mark this resource as
		// poisoned so that any downstream resources that depend on it will also be marked as poisoned and skip
		// registering while allowing the rest of the graph to continue registering.
		return makePoisonValue(res.Name()), nil
	}

	outputs, err := plugin.UnmarshalProperties(resp.Object, marshalOpts)
	if err != nil {
		return cty.NilVal, err
	}

	outputs["id"] = resource.NewProperty(resp.GetId())
	outputs["urn"] = resource.NewProperty(resp.GetUrn())
	outputs["__name"] = resource.NewProperty(request.Name)
	outputs["__type"] = resource.NewProperty(request.Type)

	// We need to ensure all schema outputs exist in the output object, even if they weren't returned by the engine.
	// - preview: unknown/computed
	// - update: explicit null
	if schemaResource != nil {
		for _, prop := range schemaResource.Properties {
			key := resource.PropertyKey(prop.Name)
			if _, ok := outputs[key]; !ok {
				if i.info.DryRun {
					outputs[key] = resource.NewProperty(resource.Computed{Element: resource.NewProperty("")})
				} else {
					outputs[key] = resource.NewNullProperty()
				}
			}
		}
	}

	result := resource.NewProperty(resource.Output{
		Element:      resource.NewProperty(outputs),
		Dependencies: []resource.URN{resource.URN(resp.GetUrn())},
		Known:        true,
	})

	return propertyValueToCty(ctx, i.monitor, result)
}

func applySchemaInputDefaults(inputs resource.PropertyMap, schemaResource *schema.Resource) {
	for _, input := range schemaResource.InputProperties {
		if input.DefaultValue == nil {
			continue
		}
		key := resource.PropertyKey(input.Name)
		if _, exists := inputs[key]; exists {
			continue
		}
		inputs[key] = resource.NewPropertyValue(input.DefaultValue.Value)
	}
}

func (i *Interpreter) registerComponent(ctx context.Context, component *pcl.Component) hcl.Diagnostics {
	inputs := resource.PropertyMap{}
	for _, attr := range component.Inputs {
		targetType := attr.Value.Type()
		if obj, ok := component.InputType.(*model.ObjectType); ok {
			if prop, ok := obj.Properties[attr.Name]; ok {
				targetType = prop
			}
		}

		expr, diags := pcl.RewriteConversions(attr.Value, targetType)
		if diags.HasErrors() {
			return diags
		}

		val, poison, diags := i.evalExpression(expr)
		if poison != nil {
			i.setRawVariable(ctx, component.Name(), makePoisonValue(*poison))
			return nil
		}
		if diags.HasErrors() {
			return diags
		}
		inputs[resource.PropertyKey(attr.Name)] = collapseResourceReferences(val)
	}

	marshalOpts := plugin.MarshalOptions{
		KeepUnknowns:  true,
		KeepSecrets:   true,
		KeepResources: true,
	}
	obj, err := plugin.MarshalProperties(inputs, marshalOpts)
	if err != nil {
		return hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "Failed to marshal component inputs",
			Detail:   err.Error(),
		}}
	}
	marshalOpts.KeepOutputValues = true

	componentName := i.effectiveName(component.LogicalName())
	request := &pulumirpc.RegisterResourceRequest{
		Type:            "components:index:" + component.DeclarationName(),
		Name:            componentName,
		Custom:          false,
		Object:          obj,
		AcceptSecrets:   true,
		AcceptResources: true,
		Parent:          i.stackURN,
	}
	if component.Options != nil && component.Options.Parent != nil {
		parent, poison, diags := i.evalExpression(component.Options.Parent)
		if poison != nil {
			i.setRawVariable(ctx, component.Name(), makePoisonValue(*poison))
			return nil
		}
		if diags.HasErrors() {
			return diags
		}
		if !parent.IsNull() && !parent.IsComputed() {
			urn, _, err := unwrapResource(parent)
			if err != nil {
				return hcl.Diagnostics{{
					Severity: hcl.DiagError,
					Summary:  "Failed to unwrap parent resource",
					Detail:   err.Error(),
				}}
			}
			request.Parent = urn
		}
	}

	resp, err := i.monitor.RegisterResource(ctx, request)
	if err != nil {
		return hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "Failed to register component",
			Detail:   err.Error(),
		}}
	}
	if resp.GetResult() != pulumirpc.Result_SUCCESS {
		return hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "Component registration failed",
			Detail:   resp.GetResult().String(),
		}}
	}

	componentEval := &hcl.EvalContext{}
	componentEval.Functions = i.evalContext.Functions
	componentEval.Variables = map[string]cty.Value{}
	componentInterpreter := &Interpreter{
		program:     component.Program,
		info:        i.info,
		monitor:     i.monitor,
		engine:      i.engine,
		loader:      i.loader,
		evalContext: componentEval,
		stackURN:    resp.GetUrn(),
		namePrefix:  componentName,
		packageRefs: i.packageRefs,
	}

	for k, v := range inputs {
		if err := componentInterpreter.setVariable(ctx, string(k), v); err != nil {
			return hcl.Diagnostics{{
				Severity: hcl.DiagError,
				Summary:  fmt.Sprintf("Failed to set component input %s", k),
				Detail:   err.Error(),
			}}
		}
	}

	componentOutputs, err := componentInterpreter.executeProgramNodes(ctx)
	if err != nil {
		return hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "Failed to execute component program nodes",
			Detail:   err.Error(),
		}}
	}
	for key, val := range componentOutputs {
		componentOutputs[key] = collapseResourceReferences(val)
	}

	outObj, err := plugin.MarshalProperties(componentOutputs, marshalOpts)
	if err != nil {
		return hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "Failed to marshal component outputs",
			Detail:   err.Error(),
		}}
	}
	_, err = i.monitor.RegisterResourceOutputs(ctx, &pulumirpc.RegisterResourceOutputsRequest{
		Urn:     resp.GetUrn(),
		Outputs: outObj,
	})
	if err != nil {
		return hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "Failed to register component outputs",
			Detail:   err.Error(),
		}}
	}

	componentObject := resource.PropertyMap{
		"id":  resource.NewProperty(resp.GetId()),
		"urn": resource.NewProperty(resp.GetUrn()),
	}
	for k, v := range componentOutputs {
		componentObject[k] = v
	}

	result := resource.NewProperty(resource.Output{
		Element:      resource.NewProperty(componentObject),
		Dependencies: []resource.URN{resource.URN(resp.GetUrn())},
		Known:        true,
	})
	if err := i.setVariable(ctx, component.Name(), result); err != nil {
		return hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "Failed to set component output" + component.Name(),
			Detail:   err.Error(),
		}}
	}
	return nil
}

func (i *Interpreter) registerStackOutputs(ctx context.Context, outputs resource.PropertyMap) error {
	if i.stackURN == "" {
		return errors.New("missing stack URN")
	}
	for key, val := range outputs {
		outputs[key] = collapseResourceReferences(val)
	}
	marshalOpts := plugin.MarshalOptions{
		KeepUnknowns:  true,
		KeepSecrets:   true,
		KeepResources: true,
	}
	obj, err := plugin.MarshalProperties(outputs, marshalOpts)
	if err != nil {
		return err
	}
	_, err = i.monitor.RegisterResourceOutputs(ctx, &pulumirpc.RegisterResourceOutputsRequest{
		Urn:     i.stackURN,
		Outputs: obj,
	})
	return err
}

func (i *Interpreter) setVariable(ctx context.Context, name string, value resource.PropertyValue) error {
	ctyValue, err := propertyValueToCty(ctx, i.monitor, value)
	if err != nil {
		return err
	}
	i.setRawVariable(ctx, name, ctyValue)
	return nil
}

func (i *Interpreter) setRawVariable(ctx context.Context, name string, value cty.Value) {
	i.evalLock.Lock()
	i.evalContext.Variables[name] = value
	i.evalLock.Unlock()
}

func parseConfigPropertyValue(raw string, typ model.Type) (resource.PropertyValue, hcl.Diagnostics) {
	ctyValue, diags := parseConfigValue(raw, typ)
	if diags.HasErrors() {
		return resource.PropertyValue{}, diags
	}
	pv, err := ctyToPropertyValue(ctyValue)
	if err != nil {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  err.Error(),
		})
	}
	return pv, diags
}

func (i *Interpreter) tryExpressions(args []cty.Value) (cty.Value, error) {
	if len(args) == 0 {
		return cty.NilVal, errors.New("at least one argument is required")
	}

	var diags hcl.Diagnostics
	for _, arg := range args {
		closure := customdecode.ExpressionClosureFromVal(arg)

		v, moreDiags := closure.Value()
		diags = append(diags, moreDiags...)

		if moreDiags.HasErrors() {
			continue
		}

		if !v.IsWhollyKnown() {
			return cty.DynamicVal, nil
		}

		pv, err := ctyToPropertyValue(v)
		if err != nil {
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  err.Error(),
			})
			continue
		}
		return propertyValueToCty(context.TODO(), i.monitor, pv)
	}

	var buf strings.Builder
	buf.WriteString("no expression succeeded:\n")
	for _, diag := range diags {
		if diag.Subject != nil {
			buf.WriteString(fmt.Sprintf("- %s (at %s)\n  %s\n", diag.Summary, diag.Subject, diag.Detail))
		} else {
			buf.WriteString(fmt.Sprintf("- %s\n  %s\n", diag.Summary, diag.Detail))
		}
	}
	buf.WriteString("\nAt least one expression must produce a successful result")
	return cty.NilVal, errors.New(buf.String())
}

func getStackOutput(stackRef resource.PropertyValue, outputName string) (resource.PropertyValue, error) {
	if stackRef.IsSecret() {
		stackRef = stackRef.SecretValue().Element
	}
	if !stackRef.IsObject() {
		return resource.NewNullProperty(), nil
	}

	obj := stackRef.ObjectValue()
	outputs, ok := obj[resource.PropertyKey("outputs")]
	if !ok {
		return resource.NewNullProperty(), nil
	}
	if outputs.IsSecret() {
		outputs = outputs.SecretValue().Element
	}
	if !outputs.IsObject() {
		return resource.NewNullProperty(), nil
	}

	outMap := outputs.ObjectValue()
	output, ok := outMap[resource.PropertyKey(outputName)]
	if !ok {
		return resource.NewNullProperty(), nil
	}

	secretByName := false
	if secretNames, ok := obj[resource.PropertyKey("secretOutputNames")]; ok {
		if secretNames.IsSecret() {
			secretNames = secretNames.SecretValue().Element
		}
		if secretNames.IsArray() {
			for _, name := range secretNames.ArrayValue() {
				if name.IsString() && name.StringValue() == outputName {
					secretByName = true
					break
				}
			}
		}
	}

	if secretByName && !output.IsSecret() {
		output = resource.MakeSecret(output)
	}

	return output, nil
}
