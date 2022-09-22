// Copyright 2016-2021, Pulumi Corporation.
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

package pulumi

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc"
)

type constructFunc func(ctx *Context, typ, name string, inputs map[string]interface{},
	options ResourceOption) (URNInput, Input, error)

// construct adapts the gRPC ConstructRequest/ConstructResponse to/from the Pulumi Go SDK programming model.
func construct(ctx context.Context, req *pulumirpc.ConstructRequest, engineConn *grpc.ClientConn,
	constructF constructFunc) (*pulumirpc.ConstructResponse, error) {

	// Configure the RunInfo.
	runInfo := RunInfo{
		Project:          req.GetProject(),
		Stack:            req.GetStack(),
		Config:           req.GetConfig(),
		ConfigSecretKeys: req.GetConfigSecretKeys(),
		Parallel:         int(req.GetParallel()),
		DryRun:           req.GetDryRun(),
		MonitorAddr:      req.GetMonitorEndpoint(),
		engineConn:       engineConn,
		Organization:     req.GetOrganization(),
	}
	pulumiCtx, err := NewContext(ctx, runInfo)
	if err != nil {
		return nil, fmt.Errorf("constructing run context: %w", err)
	}

	// Deserialize the inputs and apply appropriate dependencies.
	inputDependencies := req.GetInputDependencies()
	deserializedInputs, err := plugin.UnmarshalProperties(
		req.GetInputs(),
		plugin.MarshalOptions{
			KeepSecrets:      true,
			KeepResources:    true,
			KeepUnknowns:     req.GetDryRun(),
			KeepOutputValues: true,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling inputs: %w", err)
	}
	inputs := make(map[string]interface{}, len(deserializedInputs))
	for key, value := range deserializedInputs {
		k := string(key)
		var deps urnSet
		if inputDeps, ok := inputDependencies[k]; ok {
			deps = urnSet{}
			for _, depURN := range inputDeps.GetUrns() {
				deps.add(URN(depURN))
			}
		}

		inputs[k] = &constructInput{
			value: value,
			deps:  deps,
		}
	}

	// Rebuild the resource options.
	aliases := make([]Alias, len(req.GetAliases()))
	for i, urn := range req.GetAliases() {
		aliases[i] = Alias{URN: URN(urn)}
	}
	dependencyURNs := urnSet{}
	for _, urn := range req.GetDependencies() {
		dependencyURNs.add(URN(urn))
	}
	providers := make(map[string]ProviderResource, len(req.GetProviders()))
	for pkg, ref := range req.GetProviders() {
		resource, err := createProviderResource(pulumiCtx, ref)
		if err != nil {
			return nil, err
		}
		providers[pkg] = resource
	}
	var parent Resource
	if req.GetParent() != "" {
		parent = pulumiCtx.newDependencyResource(URN(req.GetParent()))
	}
	opts := resourceOption(func(ro *resourceOptions) {
		ro.Aliases = aliases
		ro.DependsOn = []func(ctx context.Context) (urnSet, error){
			func(ctx context.Context) (urnSet, error) {
				return dependencyURNs, nil
			},
		}
		ro.Protect = req.GetProtect()
		ro.Providers = providers
		ro.Parent = parent
	})

	urn, state, err := constructF(pulumiCtx, req.GetType(), req.GetName(), inputs, opts)
	if err != nil {
		return nil, err
	}

	// Wait for async work to finish.
	if err = pulumiCtx.wait(); err != nil {
		return nil, err
	}

	rpcURN, _, _, err := urn.ToURNOutput().awaitURN(ctx)
	if err != nil {
		return nil, err
	}

	// Serialize all state properties, first by awaiting them, and then marshaling them to the requisite gRPC values.
	resolvedProps, propertyDeps, _, err := marshalInputs(state)
	if err != nil {
		return nil, fmt.Errorf("marshaling properties: %w", err)
	}

	// Marshal all properties for the RPC call.
	keepUnknowns := req.GetDryRun()
	rpcProps, err := plugin.MarshalProperties(
		resolvedProps,
		plugin.MarshalOptions{KeepSecrets: true, KeepUnknowns: keepUnknowns, KeepResources: pulumiCtx.keepResources})
	if err != nil {
		return nil, fmt.Errorf("marshaling properties: %w", err)
	}

	// Convert the property dependencies map for RPC and remove duplicates.
	rpcPropertyDeps := make(map[string]*pulumirpc.ConstructResponse_PropertyDependencies)
	for k, deps := range propertyDeps {
		sort.Slice(deps, func(i, j int) bool { return deps[i] < deps[j] })

		urns := make([]string, 0, len(deps))
		for i, d := range deps {
			if i > 0 && urns[i-1] == string(d) {
				continue
			}
			urns = append(urns, string(d))
		}

		rpcPropertyDeps[k] = &pulumirpc.ConstructResponse_PropertyDependencies{
			Urns: urns,
		}
	}

	return &pulumirpc.ConstructResponse{
		Urn:               string(rpcURN),
		State:             rpcProps,
		StateDependencies: rpcPropertyDeps,
	}, nil
}

// createProviderResource rehydrates the provider reference into a registered ProviderResource,
// otherwise it returns an instance of DependencyProviderResource.
func createProviderResource(ctx *Context, ref string) (ProviderResource, error) {
	// Parse the URN and ID out of the provider reference.
	lastSep := strings.LastIndex(ref, "::")
	if lastSep == -1 {
		return nil, fmt.Errorf("expected '::' in provider reference %s", ref)
	}
	urn := ref[0:lastSep]
	id := ref[lastSep+2:]

	// Unmarshal the provider resource as a resource reference so we get back
	// the intended provider type with its state, if it's been registered.
	resource, err := unmarshalResourceReference(ctx, resource.ResourceReference{
		URN: resource.URN(urn),
		ID:  resource.NewStringProperty(id),
	})
	if err != nil {
		return nil, err
	}
	return resource.(ProviderResource), nil
}

type constructInput struct {
	value resource.PropertyValue
	deps  urnSet
}

func (ci constructInput) Dependencies(ctx *Context) []Resource {
	if ci.deps == nil {
		return nil
	}
	urns := ci.deps.sortedValues()
	var result []Resource
	if len(urns) > 0 {
		result = make([]Resource, len(urns))
		for i, urn := range urns {
			result[i] = ctx.newDependencyResource(urn)
		}
	}
	return result
}

// constructInputsMap returns the inputs as a Map.
func constructInputsMap(ctx *Context, inputs map[string]interface{}) (Map, error) {
	result := make(Map, len(inputs))
	for k, v := range inputs {
		ci := v.(*constructInput)

		known := !ci.value.ContainsUnknowns()
		value, secret, err := unmarshalPropertyValue(ctx, ci.value)
		if err != nil {
			return nil, fmt.Errorf("unmarshaling input %q: %w", k, err)
		}

		resultType := anyOutputType
		if ot, ok := concreteTypeToOutputType.Load(reflect.TypeOf(value)); ok {
			resultType = ot.(reflect.Type)
		}

		output := ctx.newOutput(resultType, ci.Dependencies(ctx)...)
		output.getState().resolve(value, known, secret, nil)
		result[k] = output
	}
	return result, nil
}

func gatherDeps(v resource.PropertyValue, deps urnSet) {
	switch {
	case v.IsSecret():
		gatherDeps(v.SecretValue().Element, deps)
	case v.IsComputed():
		gatherDeps(v.Input().Element, deps)
	case v.IsOutput():
		for _, urn := range v.OutputValue().Dependencies {
			deps.add(URN(urn))
		}
		gatherDeps(v.OutputValue().Element, deps)
	case v.IsResourceReference():
		deps.add(URN(v.ResourceReferenceValue().URN))
	case v.IsArray():
		for _, e := range v.ArrayValue() {
			gatherDeps(e, deps)
		}
	case v.IsObject():
		for _, e := range v.ObjectValue() {
			gatherDeps(e, deps)
		}
	}
}

func copyInputTo(ctx *Context, v resource.PropertyValue, dest reflect.Value) error {
	contract.Assert(dest.CanSet())

	// Check for nils. The destination will be left with the zero value.
	if v.IsNull() {
		return nil
	}

	// Allocate storage as necessary.
	for dest.Kind() == reflect.Ptr {
		elem := reflect.New(dest.Type().Elem())
		dest.Set(elem)
		dest = elem.Elem()
	}

	switch {
	case v.IsSecret():
		known, element := true, v.SecretValue().Element
		// See if it's value is unknown.
		if element.IsComputed() {
			known, element = false, element.Input().Element
		}
		// Handle this as a secret output.
		return copyInputTo(ctx, resource.NewOutputProperty(resource.Output{
			Element: element,
			Known:   known,
			Secret:  true,
		}), dest)
	case v.IsComputed():
		// Handle this as an unknown output.
		return copyInputTo(ctx, resource.MakeOutput(v.Input().Element), dest)
	case v.IsOutput():
		// If it's known, there aren't any dependencies, and it's not a secret, just copy the value.
		if v.OutputValue().Known && len(v.OutputValue().Dependencies) == 0 && !v.OutputValue().Secret {
			return copyInputTo(ctx, v.OutputValue().Element, dest)
		}

		if !dest.Type().Implements(outputType) && !dest.Type().Implements(inputType) {
			return fmt.Errorf("expected destination type to implement %v or %v, got %v", inputType, outputType, dest.Type())
		}

		resourceDeps := make([]Resource, len(v.OutputValue().Dependencies))
		for i, dep := range v.OutputValue().Dependencies {
			resourceDeps[i] = ctx.newDependencyResource(URN(dep))
		}

		result, err := createOutput(ctx, dest.Type(), v.OutputValue().Element, v.OutputValue().Known,
			v.OutputValue().Secret, resourceDeps)
		if err != nil {
			return err
		}
		dest.Set(result)
		return nil
	}

	if dest.Type().Implements(outputType) {
		result, err := createOutput(ctx, dest.Type(), v, true /*known*/, false /*secret*/, nil)
		if err != nil {
			return err
		}
		dest.Set(result)
		return nil
	}

	if dest.Type().Implements(inputType) {
		// Try to determine the input type from the interface.
		if it, ok := inputInterfaceTypeToConcreteType.Load(dest.Type()); ok {
			inputType := it.(reflect.Type)

			for inputType.Kind() == reflect.Ptr {
				inputType = inputType.Elem()
			}

			switch inputType.Kind() {
			case reflect.Bool:
				if !v.IsBool() {
					return fmt.Errorf("expected a %v, got a %s", inputType, v.TypeString())
				}
				result := reflect.New(inputType).Elem()
				result.SetBool(v.BoolValue())
				dest.Set(result)
				return nil
			case reflect.Int:
				if !v.IsNumber() {
					return fmt.Errorf("expected an %v, got a %s", inputType, v.TypeString())
				}
				result := reflect.New(inputType).Elem()
				result.SetInt(int64(v.NumberValue()))
				dest.Set(result)
				return nil
			case reflect.Float64:
				if !v.IsNumber() {
					return fmt.Errorf("expected an %v, got a %s", inputType, v.TypeString())
				}
				result := reflect.New(inputType).Elem()
				result.SetFloat(v.NumberValue())
				dest.Set(result)
				return nil
			case reflect.String:
				if !v.IsString() {
					return fmt.Errorf("expected a %v, got a %s", inputType, v.TypeString())
				}
				result := reflect.New(inputType).Elem()
				result.SetString(v.StringValue())
				dest.Set(result)
				return nil
			case reflect.Slice:
				return copyToSlice(ctx, v, inputType, dest)
			case reflect.Map:
				return copyToMap(ctx, v, inputType, dest)
			case reflect.Interface:
				if !anyType.Implements(inputType) {
					return fmt.Errorf("cannot unmarshal into non-empty interface type %v", inputType)
				}
				result, _, err := unmarshalPropertyValue(ctx, v)
				if err != nil {
					return err
				}
				dest.Set(reflect.ValueOf(result))
				return nil
			case reflect.Struct:
				return copyToStruct(ctx, v, inputType, dest)
			default:
				panic(fmt.Sprintf("%v", inputType.Kind()))
			}
		}

		if v.IsAsset() {
			if !assetType.AssignableTo(dest.Type()) {
				return fmt.Errorf("expected a %s, got an asset", dest.Type())
			}
			asset, _, err := unmarshalPropertyValue(ctx, v)
			if err != nil {
				return err
			}
			dest.Set(reflect.ValueOf(asset))
			return nil
		}

		if v.IsArchive() {
			if !archiveType.AssignableTo(dest.Type()) {
				return fmt.Errorf("expected a %s, got an archive", dest.Type())
			}
			archive, _, err := unmarshalPropertyValue(ctx, v)
			if err != nil {
				return err
			}
			dest.Set(reflect.ValueOf(archive))
			return nil
		}

		// For plain structs that implements the Input interface.
		if dest.Type().Kind() == reflect.Struct {
			return copyToStruct(ctx, v, dest.Type(), dest)
		}

		// Otherwise, create an output.
		result, err := createOutput(ctx, dest.Type(), v, true /*known*/, false /*secret*/, nil)
		if err != nil {
			return err
		}
		dest.Set(result)
		return nil
	}

	// A resource reference looks like a struct, but must be deserialzed differently.
	if !v.IsResourceReference() {
		switch dest.Type().Kind() {
		case reflect.Map:
			return copyToMap(ctx, v, dest.Type(), dest)
		case reflect.Slice:
			return copyToSlice(ctx, v, dest.Type(), dest)
		case reflect.Struct:
			return copyToStruct(ctx, v, dest.Type(), dest)
		}
	}

	_, err := unmarshalOutput(ctx, v, dest)
	if err != nil {
		return fmt.Errorf("unmarshaling value: %w", err)
	}
	return nil
}

func createOutput(ctx *Context, destType reflect.Type, v resource.PropertyValue, known, secret bool,
	deps []Resource) (reflect.Value, error) {

	outputType := getOutputType(destType)
	output := ctx.newOutput(outputType, deps...)
	outputValueDest := reflect.New(output.ElementType()).Elem()
	_, err := unmarshalOutput(ctx, v, outputValueDest)
	if err != nil {
		return reflect.Value{}, fmt.Errorf("unmarshaling value: %w", err)
	}
	output.getState().resolve(outputValueDest.Interface(), known, secret, nil)
	return reflect.ValueOf(output), nil
}

func getOutputType(typ reflect.Type) reflect.Type {
	if typ.Implements(outputType) {
		return typ
	} else if typ.Implements(inputType) && typ.Kind() == reflect.Interface {
		// Attempt to determine the output type by looking up the registered input type,
		// getting the input type's element type, and then looking up the registered output
		// type by the element type.
		if inputStructType, found := inputInterfaceTypeToConcreteType.Load(typ); found {
			input := reflect.New(inputStructType.(reflect.Type)).Elem().Interface().(Input)
			elementType := input.ElementType()
			if outputType, ok := concreteTypeToOutputType.Load(elementType); ok {
				return outputType.(reflect.Type)
			}
		}

		// Otherwise, look for a `To<Name>Output` method that returns an output.
		toOutputMethodName := "To" + strings.TrimSuffix(typ.Name(), "Input") + "Output"
		if toOutputMethod, found := typ.MethodByName(toOutputMethodName); found {
			mt := toOutputMethod.Type
			if mt.NumIn() == 0 && mt.NumOut() == 1 && mt.Out(0).Implements(outputType) {
				return mt.Out(0)
			}
		}
	}
	return anyOutputType
}

func copyToSlice(ctx *Context, v resource.PropertyValue, typ reflect.Type, dest reflect.Value) error {
	if !v.IsArray() {
		return fmt.Errorf("expected a %v, got a %s", typ, v.TypeString())
	}
	arr := v.ArrayValue()
	slice := reflect.MakeSlice(typ, len(arr), len(arr))
	for i, e := range arr {
		if err := copyInputTo(ctx, e, slice.Index(i)); err != nil {
			return err
		}
	}
	dest.Set(slice)
	return nil
}

func copyToMap(ctx *Context, v resource.PropertyValue, typ reflect.Type, dest reflect.Value) error {
	if !v.IsObject() {
		return fmt.Errorf("expected a %v, got a %s", typ, v.TypeString())
	}

	keyType, elemType := typ.Key(), typ.Elem()
	if keyType.Kind() != reflect.String {
		return fmt.Errorf("map keys must be assignable from type string")
	}

	result := reflect.MakeMap(typ)
	for k, e := range v.ObjectValue() {
		if resource.IsInternalPropertyKey(k) {
			continue
		}
		elem := reflect.New(elemType).Elem()
		if err := copyInputTo(ctx, e, elem); err != nil {
			return err
		}

		key := reflect.New(keyType).Elem()
		key.SetString(string(k))

		result.SetMapIndex(key, elem)
	}
	dest.Set(result)
	return nil
}

func copyToStruct(ctx *Context, v resource.PropertyValue, typ reflect.Type, dest reflect.Value) error {
	if !v.IsObject() {
		return fmt.Errorf("expected a %v, got a %s", typ, v.TypeString())
	}
	result := reflect.New(typ).Elem()

	obj := v.ObjectValue()
	for i := 0; i < typ.NumField(); i++ {
		fieldV := result.Field(i)
		if !fieldV.CanSet() {
			continue
		}

		tag := typ.Field(i).Tag.Get("pulumi")
		if tag == "" {
			continue
		}

		e, ok := obj[resource.PropertyKey(tag)]
		if !ok {
			continue
		}

		if err := copyInputTo(ctx, e, fieldV); err != nil {
			return err
		}
	}
	dest.Set(result)
	return nil
}

// constructInputsCopyTo sets the inputs on the given args struct.
func constructInputsCopyTo(ctx *Context, inputs map[string]interface{}, args interface{}) error {
	if args == nil {
		return errors.New("args must not be nil")
	}
	argsV := reflect.ValueOf(args)
	typ := argsV.Type()
	if typ.Kind() != reflect.Ptr || typ.Elem().Kind() != reflect.Struct {
		return errors.New("args must be a pointer to a struct")
	}
	argsV, typ = argsV.Elem(), typ.Elem()

	for k, v := range inputs {
		ci := v.(*constructInput)
		for i := 0; i < typ.NumField(); i++ {
			fieldV := argsV.Field(i)
			if !fieldV.CanSet() {
				continue
			}
			field := typ.Field(i)
			tag, has := field.Tag.Lookup("pulumi")
			tag = strings.Split(tag, ",")[0] // tagName,flag => tagName
			if !has || tag != k {
				continue
			}

			// Find all nested dependencies.
			deps := urnSet{}
			gatherDeps(ci.value, deps)

			// If the top-level property dependencies are equal to (or a subset of) the gathered nested
			// dependencies, we don't necessarily need to create a top-level output for the property.
			if deps.contains(ci.deps) {
				if err := copyInputTo(ctx, ci.value, fieldV); err != nil {
					return fmt.Errorf("copying input %q: %w", k, err)
				}
				continue
			}

			handleField := func(typ reflect.Type, value resource.PropertyValue,
				deps []Resource) (reflect.Value, error) {
				resultType := getOutputType(typ)
				output := ctx.newOutput(resultType, deps...)
				dest := reflect.New(output.ElementType()).Elem()
				known := !ci.value.ContainsUnknowns()
				secret, err := unmarshalOutput(ctx, value, dest)
				if err != nil {
					return reflect.Value{}, err
				}
				output.getState().resolve(dest.Interface(), known, secret, nil)
				return reflect.ValueOf(output), nil
			}

			isOutputOrInputType := func(typ reflect.Type) bool {
				return typ.Implements(outputType) || typ.Implements(inputType)
			}

			if isOutputOrInputType(field.Type) {
				val, err := handleField(field.Type, ci.value, ci.Dependencies(ctx))
				if err != nil {
					return err
				}
				fieldV.Set(val)
				continue
			}

			if field.Type.Kind() == reflect.Slice && isOutputOrInputType(field.Type.Elem()) {
				elemType := field.Type.Elem()
				length := len(ci.value.ArrayValue())
				dest := reflect.MakeSlice(field.Type, length, length)
				for i := 0; i < length; i++ {
					val, err := handleField(elemType, ci.value.ArrayValue()[i], ci.Dependencies(ctx))
					if err != nil {
						return err
					}
					dest.Index(i).Set(val)
				}
				fieldV.Set(dest)
				continue
			}

			if field.Type.Kind() == reflect.Map && isOutputOrInputType(field.Type.Elem()) {
				elemType := field.Type.Elem()
				length := len(ci.value.ObjectValue())
				dest := reflect.MakeMapWithSize(field.Type, length)
				for k, v := range ci.value.ObjectValue() {
					key := reflect.ValueOf(string(k))
					val, err := handleField(elemType, v, ci.Dependencies(ctx))
					if err != nil {
						return err
					}
					dest.SetMapIndex(key, val)
				}
				fieldV.Set(dest)
				continue
			}

			if len(ci.deps) > 0 {
				return fmt.Errorf("copying input %q: %s.%s is typed as %v but must be a type that implements %v or "+
					"%v for input with dependencies", k, typ, field.Name, field.Type, inputType, outputType)
			}
			dest := reflect.New(field.Type).Elem()
			secret, err := unmarshalOutput(ctx, ci.value, dest)
			if err != nil {
				return fmt.Errorf("copying input %q: unmarshaling value: %w", k, err)
			}
			if secret {
				return fmt.Errorf("copying input %q: %s.%s is typed as %v but must be a type that implements %v or "+
					"%v for secret input", k, typ, field.Name, field.Type, inputType, outputType)
			}
			fieldV.Set(reflect.ValueOf(dest.Interface()))
		}
	}

	return nil
}

// newConstructResult converts a resource into its associated URN and state.
func newConstructResult(resource ComponentResource) (URNInput, Input, error) {
	if resource == nil {
		return nil, nil, errors.New("resource must not be nil")
	}

	resourceV := reflect.ValueOf(resource)
	typ := resourceV.Type()
	if typ.Kind() != reflect.Ptr || typ.Elem().Kind() != reflect.Struct {
		return nil, nil, errors.New("resource must be a pointer to a struct")
	}
	resourceV, typ = resourceV.Elem(), typ.Elem()

	state := make(Map)
	for i := 0; i < typ.NumField(); i++ {
		fieldV := resourceV.Field(i)
		if !fieldV.CanInterface() {
			continue
		}
		field := typ.Field(i)
		tag, has := field.Tag.Lookup("pulumi")
		if !has {
			continue
		}
		val := fieldV.Interface()
		if v, ok := val.(Input); ok {
			state[tag] = v
		} else {
			state[tag] = ToOutput(val)
		}
	}

	return resource.URN(), state, nil
}

// callFailure indicates that a call to Call failed; it contains the property and reason for the failure.
type callFailure struct {
	Property string
	Reason   string
}

type callFunc func(ctx *Context, tok string, args map[string]interface{}) (Input, []interface{}, error)

// call adapts the gRPC CallRequest/CallResponse to/from the Pulumi Go SDK programming model.
func call(ctx context.Context, req *pulumirpc.CallRequest, engineConn *grpc.ClientConn,
	callF callFunc) (*pulumirpc.CallResponse, error) {

	// Configure the RunInfo.
	runInfo := RunInfo{
		Project:      req.GetProject(),
		Stack:        req.GetStack(),
		Config:       req.GetConfig(),
		Parallel:     int(req.GetParallel()),
		DryRun:       req.GetDryRun(),
		MonitorAddr:  req.GetMonitorEndpoint(),
		engineConn:   engineConn,
		Organization: req.GetOrganization(),
	}
	pulumiCtx, err := NewContext(ctx, runInfo)
	if err != nil {
		return nil, fmt.Errorf("constructing run context: %w", err)
	}

	// Deserialize the inputs and apply appropriate dependencies.
	argDependencies := req.GetArgDependencies()
	deserializedArgs, err := plugin.UnmarshalProperties(
		req.GetArgs(),
		plugin.MarshalOptions{
			KeepSecrets:      true,
			KeepResources:    true,
			KeepUnknowns:     req.GetDryRun(),
			KeepOutputValues: true,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling inputs: %w", err)
	}
	args := make(map[string]interface{}, len(deserializedArgs))
	for key, value := range deserializedArgs {
		k := string(key)
		var deps urnSet
		if inputDeps, ok := argDependencies[k]; ok {
			deps = urnSet{}
			for _, depURN := range inputDeps.GetUrns() {
				deps.add(URN(depURN))
			}
		}

		args[k] = &constructInput{
			value: value,
			deps:  deps,
		}
	}

	result, failures, err := callF(pulumiCtx, req.GetTok(), args)
	if err != nil {
		return nil, err
	}

	// Wait for async work to finish.
	if err = pulumiCtx.wait(); err != nil {
		return nil, err
	}

	// Serialize all result properties, first by awaiting them, and then marshaling them to the requisite gRPC values.
	resolvedProps, propertyDeps, _, err := marshalInputs(result)
	if err != nil {
		return nil, fmt.Errorf("marshaling properties: %w", err)
	}

	// Marshal all properties for the RPC call.
	keepUnknowns := req.GetDryRun()
	rpcProps, err := plugin.MarshalProperties(
		resolvedProps,
		plugin.MarshalOptions{KeepSecrets: true, KeepUnknowns: keepUnknowns, KeepResources: pulumiCtx.keepResources})
	if err != nil {
		return nil, fmt.Errorf("marshaling properties: %w", err)
	}

	// Convert the property dependencies map for RPC and remove duplicates.
	rpcPropertyDeps := make(map[string]*pulumirpc.CallResponse_ReturnDependencies)
	for k, deps := range propertyDeps {
		sort.Slice(deps, func(i, j int) bool { return deps[i] < deps[j] })

		urns := make([]string, 0, len(deps))
		for i, d := range deps {
			if i > 0 && urns[i-1] == string(d) {
				continue
			}
			urns = append(urns, string(d))
		}

		rpcPropertyDeps[k] = &pulumirpc.CallResponse_ReturnDependencies{
			Urns: urns,
		}
	}

	var rpcFailures []*pulumirpc.CheckFailure
	if len(failures) > 0 {
		rpcFailures = make([]*pulumirpc.CheckFailure, len(failures))
		for i, v := range failures {
			failure := v.(callFailure)
			rpcFailures[i] = &pulumirpc.CheckFailure{
				Property: failure.Property,
				Reason:   failure.Reason,
			}
		}
	}

	return &pulumirpc.CallResponse{
		Return:             rpcProps,
		ReturnDependencies: rpcPropertyDeps,
		Failures:           rpcFailures,
	}, nil
}

// callArgsCopyTo sets the args on the given args struct. If there is a `__self__` argument, it will be
// returned, otherwise it will return nil.
func callArgsCopyTo(ctx *Context, source map[string]interface{}, args interface{}) (Resource, error) {
	// Use the same implementation as construct.
	if err := constructInputsCopyTo(ctx, source, args); err != nil {
		return nil, err
	}

	// Retrieve the `__self__` arg.
	self, err := callArgsSelf(ctx, source)
	if err != nil {
		return nil, err
	}

	return self, nil
}

// callArgsSelf retrieves the `__self__` argument. If `__self__` is present the value is returned,
// otherwise the returned value will be nil.
func callArgsSelf(ctx *Context, source map[string]interface{}) (Resource, error) {
	v, ok := source["__self__"]
	if !ok {
		return nil, nil
	}

	ci := v.(*constructInput)
	if ci.value.ContainsUnknowns() {
		return nil, errors.New("__self__ is unknown")
	}

	value, secret, err := unmarshalPropertyValue(ctx, ci.value)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling __self__: %w", err)
	}
	if secret {
		return nil, errors.New("__self__ is a secret")
	}

	return value.(Resource), nil
}

// newCallResult converts a result struct into an input Map that can be marshalled.
func newCallResult(result interface{}) (Input, error) {
	if result == nil {
		return nil, errors.New("result must not be nil")
	}

	resultV := reflect.ValueOf(result)
	typ := resultV.Type()
	if typ.Kind() != reflect.Ptr || typ.Elem().Kind() != reflect.Struct {
		return nil, errors.New("result must be a pointer to a struct")
	}
	resultV, typ = resultV.Elem(), typ.Elem()

	ret := make(Map)
	for i := 0; i < typ.NumField(); i++ {
		fieldV := resultV.Field(i)
		if !fieldV.CanInterface() {
			continue
		}
		field := typ.Field(i)
		tag, has := field.Tag.Lookup("pulumi")
		if !has {
			continue
		}
		val := fieldV.Interface()
		if v, ok := val.(Input); ok {
			ret[tag] = v
		} else {
			ret[tag] = ToOutput(val)
		}
	}

	return ret, nil
}

// newCallFailure creates a call failure.
func newCallFailure(property, reason string) interface{} {
	return callFailure{
		Property: property,
		Reason:   reason,
	}
}
