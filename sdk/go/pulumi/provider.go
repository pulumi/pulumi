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
	"reflect"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"

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
	}
	pulumiCtx, err := NewContext(ctx, runInfo)
	if err != nil {
		return nil, errors.Wrap(err, "constructing run context")
	}

	// Deserialize the inputs and apply appropriate dependencies.
	inputDependencies := req.GetInputDependencies()
	deserializedInputs, err := plugin.UnmarshalProperties(
		req.GetInputs(),
		plugin.MarshalOptions{KeepSecrets: true, KeepResources: true, KeepUnknowns: req.GetDryRun()},
	)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshaling inputs")
	}
	inputs := make(map[string]interface{}, len(deserializedInputs))
	for key, value := range deserializedInputs {
		k := string(key)
		var deps []Resource
		if inputDeps, ok := inputDependencies[k]; ok {
			deps = make([]Resource, len(inputDeps.GetUrns()))
			for i, depURN := range inputDeps.GetUrns() {
				deps[i] = pulumiCtx.newDependencyResource(URN(depURN))
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
	dependencies := make([]Resource, len(req.GetDependencies()))
	for i, urn := range req.GetDependencies() {
		dependencies[i] = pulumiCtx.newDependencyResource(URN(urn))
	}
	providers := make(map[string]ProviderResource, len(req.GetProviders()))
	for pkg, ref := range req.GetProviders() {
		// Parse the URN and ID out of the provider reference.
		lastSep := strings.LastIndex(ref, "::")
		if lastSep == -1 {
			return nil, errors.Errorf("expected '::' in provider reference %s", ref)
		}
		urn := ref[0:lastSep]
		id := ref[lastSep+2:]
		providers[pkg] = pulumiCtx.newDependencyProviderResource(URN(urn), ID(id))
	}
	var parent Resource
	if req.GetParent() != "" {
		parent = pulumiCtx.newDependencyResource(URN(req.GetParent()))
	}
	opts := resourceOption(func(ro *resourceOptions) {
		ro.Aliases = aliases
		ro.DependsOn = dependencies
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
		return nil, errors.Wrap(err, "marshaling properties")
	}

	// Marshal all properties for the RPC call.
	keepUnknowns := req.GetDryRun()
	rpcProps, err := plugin.MarshalProperties(
		resolvedProps,
		plugin.MarshalOptions{KeepSecrets: true, KeepUnknowns: keepUnknowns, KeepResources: pulumiCtx.keepResources})
	if err != nil {
		return nil, errors.Wrap(err, "marshaling properties")
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

type constructInput struct {
	value resource.PropertyValue
	deps  []Resource
}

// constructInputsMap returns the inputs as a Map.
func constructInputsMap(ctx *Context, inputs map[string]interface{}) (Map, error) {
	result := make(Map, len(inputs))
	for k, v := range inputs {
		ci := v.(*constructInput)

		known := !ci.value.ContainsUnknowns()
		value, secret, err := unmarshalPropertyValue(ctx, ci.value)
		if err != nil {
			return nil, errors.Wrapf(err, "unmarshaling input %s", k)
		}

		resultType := anyOutputType
		if ot, ok := concreteTypeToOutputType.Load(reflect.TypeOf(value)); ok {
			resultType = ot.(reflect.Type)
		}

		output := ctx.newOutput(resultType, ci.deps...)
		output.getState().resolve(value, known, secret, nil)
		result[k] = output
	}
	return result, nil
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
			if !has || tag != k {
				continue
			}

			handleField := func(typ reflect.Type, value resource.PropertyValue, deps []Resource) (reflect.Value, error) {
				resultType := anyOutputType
				if typ.Implements(outputType) {
					resultType = typ
				} else if typ.Implements(inputType) {
					toOutputMethodName := "To" + strings.TrimSuffix(typ.Name(), "Input") + "Output"
					if toOutputMethod, found := typ.MethodByName(toOutputMethodName); found {
						mt := toOutputMethod.Type
						if mt.NumIn() == 0 && mt.NumOut() == 1 && mt.Out(0).Implements(outputType) {
							resultType = mt.Out(0)
						}
					}
				}
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

			isInputType := func(typ reflect.Type) bool {
				return typ.Implements(outputType) || typ.Implements(inputType)
			}

			if isInputType(field.Type) {
				val, err := handleField(field.Type, ci.value, ci.deps)
				if err != nil {
					return err
				}
				fieldV.Set(val)
				continue
			}

			if field.Type.Kind() == reflect.Slice && isInputType(field.Type.Elem()) {
				elemType := field.Type.Elem()
				length := len(ci.value.ArrayValue())
				dest := reflect.MakeSlice(field.Type, length, length)
				for i := 0; i < length; i++ {
					val, err := handleField(elemType, ci.value.ArrayValue()[i], ci.deps)
					if err != nil {
						return err
					}
					dest.Index(i).Set(val)
				}
				fieldV.Set(dest)
				continue
			}

			if field.Type.Kind() == reflect.Map && isInputType(field.Type.Elem()) {
				elemType := field.Type.Elem()
				length := len(ci.value.ObjectValue())
				dest := reflect.MakeMapWithSize(field.Type, length)
				for k, v := range ci.value.ObjectValue() {
					key := reflect.ValueOf(string(k))
					val, err := handleField(elemType, v, ci.deps)
					if err != nil {
						return err
					}
					dest.SetMapIndex(key, val)
				}
				fieldV.Set(dest)
				continue
			}

			if len(ci.deps) > 0 {
				return errors.Errorf(
					"%s.%s is typed as %v but must be typed as Input or Output for input %q with dependencies",
					typ, field.Name, field.Type, k)
			}
			dest := reflect.New(field.Type).Elem()
			secret, err := unmarshalOutput(ctx, ci.value, dest)
			if err != nil {
				return errors.Wrapf(err, "unmarshaling input %s", k)
			}
			if secret {
				return errors.Errorf(
					"%s.%s is typed as %v but must be typed as Input or Output for secret input %q",
					typ, field.Name, field.Type, k)
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

type callFunc func(ctx *Context, tok string, args map[string]interface{}) (Input, error)

// call adapts the gRPC CallRequest/CallResponse to/from the Pulumi Go SDK programming model.
func call(ctx context.Context, req *pulumirpc.CallRequest, engineConn *grpc.ClientConn,
	callF callFunc) (*pulumirpc.CallResponse, error) {

	// Configure the RunInfo.
	runInfo := RunInfo{
		Project:     req.GetProject(),
		Stack:       req.GetStack(),
		Config:      req.GetConfig(),
		Parallel:    int(req.GetParallel()),
		DryRun:      req.GetDryRun(),
		MonitorAddr: req.GetMonitorEndpoint(),
		engineConn:  engineConn,
	}
	pulumiCtx, err := NewContext(ctx, runInfo)
	if err != nil {
		return nil, errors.Wrap(err, "constructing run context")
	}

	// Deserialize the inputs and apply appropriate dependencies.
	argDependencies := req.GetArgDependencies()
	deserializedArgs, err := plugin.UnmarshalProperties(
		req.GetArgs(),
		plugin.MarshalOptions{KeepSecrets: true, KeepResources: true, KeepUnknowns: req.GetDryRun()},
	)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshaling inputs")
	}
	args := make(map[string]interface{}, len(deserializedArgs))
	for key, value := range deserializedArgs {
		k := string(key)
		var deps []Resource
		if inputDeps, ok := argDependencies[k]; ok {
			deps = make([]Resource, len(inputDeps.GetUrns()))
			for i, depURN := range inputDeps.GetUrns() {
				deps[i] = pulumiCtx.newDependencyResource(URN(depURN))
			}
		}

		args[k] = &constructInput{
			value: value,
			deps:  deps,
		}
	}

	result, err := callF(pulumiCtx, req.GetTok(), args)
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
		return nil, errors.Wrap(err, "marshaling properties")
	}

	// Marshal all properties for the RPC call.
	keepUnknowns := req.GetDryRun()
	rpcProps, err := plugin.MarshalProperties(
		resolvedProps,
		plugin.MarshalOptions{KeepSecrets: true, KeepUnknowns: keepUnknowns, KeepResources: pulumiCtx.keepResources})
	if err != nil {
		return nil, errors.Wrap(err, "marshaling properties")
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

	return &pulumirpc.CallResponse{
		Return:             rpcProps,
		ReturnDependencies: rpcPropertyDeps,
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
		return nil, errors.Wrap(err, "unmarshaling __self__")
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
