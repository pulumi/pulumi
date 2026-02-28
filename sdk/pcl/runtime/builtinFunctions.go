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
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/hcl/v2/ext/customdecode"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/archive"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/asset"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

func (i *Interpreter) builtinFunctions() map[string]function.Function {
	stringFn := func(value string) function.Function {
		return function.New(&function.Spec{
			Params: []function.Parameter{},
			Type:   function.StaticReturnType(cty.String),
			Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
				return propertyValueToCty(resource.NewProperty(value))
			},
		})
	}

	secretFn := function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name:        "value",
				Type:        cty.DynamicPseudoType,
				AllowMarked: true,
			},
		},
		Type: func(args []cty.Value) (cty.Type, error) {
			if len(args) == 0 {
				return cty.DynamicPseudoType, nil
			}
			return args[0].Type(), nil
		},
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			if len(args) == 0 {
				return cty.NilVal, errors.New("secret requires a value")
			}
			return args[0].Mark(secretMark{}), nil
		},
	})

	unsecretFn := function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name:        "value",
				Type:        cty.DynamicPseudoType,
				AllowMarked: true,
			},
		},
		Type: func(args []cty.Value) (cty.Type, error) {
			if len(args) == 0 {
				return cty.DynamicPseudoType, nil
			}
			return args[0].Type(), nil
		},
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			if len(args) == 0 {
				return cty.NilVal, errors.New("unsecret requires a value")
			}
			val, _ := unmark[secretMark](args[0])
			return val, nil
		},
	})

	tryFn := function.New(&function.Spec{
		VarParam: &function.Parameter{
			Name: "expressions",
			Type: customdecode.ExpressionClosureType,
		},
		Type: func(args []cty.Value) (cty.Type, error) {
			v, err := tryExpressions(args)
			if err != nil {
				return cty.NilType, err
			}
			return v.Type(), nil
		},
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			return tryExpressions(args)
		},
	})

	canFn := function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "expression",
				Type: customdecode.ExpressionClosureType,
			},
		},
		Type: function.StaticReturnType(cty.Bool),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			if len(args) == 0 {
				return cty.NilVal, errors.New("can requires an expression")
			}
			closure := customdecode.ExpressionClosureFromVal(args[0])
			value, diags := closure.Value()
			if diags.HasErrors() {
				return cty.False, nil
			}
			if !value.IsWhollyKnown() {
				return cty.UnknownVal(cty.Bool), nil
			}
			pv, err := ctyToPropertyValue(value)
			if err != nil {
				return cty.False, nil
			}
			if pv.IsSecret() {
				return propertyValueToCty(resource.MakeSecret(resource.NewProperty(true)))
			}
			return cty.True, nil
		},
	})

	getOutputFn := function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "stackReference",
				Type: cty.DynamicPseudoType,
			},
			{
				Name: "outputName",
				Type: cty.String,
			},
		},
		Type: function.StaticReturnType(cty.DynamicPseudoType),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			if len(args) != 2 {
				return cty.NilVal, errors.New("getOutput requires a stack reference and output name")
			}

			stackRefPV, err := ctyToPropertyValue(args[0])
			if err != nil {
				return cty.DynamicVal, nil
			}
			if stackRefPV.IsNull() {
				return cty.DynamicVal, nil
			}
			if args[1].IsNull() || !args[1].IsKnown() {
				return cty.DynamicVal, nil
			}
			outputName := args[1].AsString()

			outputPV, err := getStackOutput(stackRefPV, outputName)
			if err != nil {
				return cty.NilVal, err
			}
			return propertyValueToCty(outputPV)
		},
	})

	invokeFn := function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "token",
				Type: cty.String,
			},
			{
				Name: "args",
				Type: cty.DynamicPseudoType,
			},
		},
		VarParam: &function.Parameter{
			Name:      "options",
			Type:      invokeOptionsType,
			AllowNull: true,
		},
		Type: function.StaticReturnType(cty.DynamicPseudoType),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			if len(args) < 2 {
				return cty.NilVal, errors.New("invoke requires a token and arguments")
			}
			if len(args) > 3 {
				return cty.NilVal, errors.New("invoke accepts at most three arguments: token, arguments, and options")
			}
			if args[0].Type() != cty.String {
				return cty.NilVal, errors.New("invoke token must be a string")
			}
			token := args[0].AsString()
			components := strings.Split(token, ":")
			contract.Assertf(len(components) == 3, "invalid token format: %s", token)
			if components[1] == "" {
				components[1] = "index"
			}
			token = fmt.Sprintf("%s:%s:%s", components[0], components[1], components[2])

			// Fall back to just the package name and passed in version if we don't have a descriptor.
			descriptor := &schema.PackageDescriptor{
				Name: components[0],
			}
			pkg, err := i.loader.LoadPackageReferenceV2(context.TODO(), descriptor)
			if err != nil {
				return cty.NilVal, fmt.Errorf("load package for token %s: %w", token, err)
			}
			functions := pkg.Functions()
			fun, ok, err := functions.Get(token)
			if err != nil {
				return cty.NilVal, fmt.Errorf("get function from package for token %s: %w", token, err)
			}
			if !ok {
				// Didn't find the function via a direct lookup, we now need to iterate _all_ the functions and use
				// TokenToModule to see if any of the match the token we have.
				iter := functions.Range()
				for iter.Next() {
					fnToken := iter.Token()
					// Canoncalise the functions token via TokenToModule
					mod := pkg.TokenToModule(fnToken)
					components := strings.Split(fnToken, ":")
					fnToken = fmt.Sprintf("%s:%s:%s", components[0], mod, components[2])
					if token == fnToken {
						fun, err = iter.Function()
						if err != nil {
							return cty.NilVal, fmt.Errorf("get function from package for token %s: %w", token, err)
						}
						token = iter.Token()
						break
					}
				}
			}

			argsPV, err := ctyToPropertyValue(args[1])
			if err != nil {
				return cty.NilVal, fmt.Errorf("invalid invoke arguments: %w", err)
			}

			marshalOpts := plugin.MarshalOptions{
				KeepUnknowns:  true,
				KeepSecrets:   true,
				KeepResources: true,
			}
			obj, err := plugin.MarshalProperties(argsPV.ObjectValue(), marshalOpts)
			if err != nil {
				return cty.NilVal, fmt.Errorf("marshal invoke arguments: %w", err)
			}

			request := &pulumirpc.ResourceInvokeRequest{
				Tok:  token,
				Args: obj,
			}

			var dependsOn []resource.URN
			if len(args) == 3 && !args[2].IsNull() {
				options := args[2]

				deps := options.GetAttr("dependsOn")
				if !deps.IsNull() && deps.IsKnown() {
					if !deps.Type().IsListType() {
						return cty.NilVal, errors.New("invoke options dependsOn must be a list of strings")
					}
					for it := deps.ElementIterator(); it.Next(); {
						_, dep := it.Element()
						if dep.IsNull() || !dep.IsKnown() {
							continue
						}
						// dependencies should be resource objects that will have a urn property
						if !dep.Type().IsObjectType() {
							return cty.NilVal, errors.New("invoke options dependsOn must be a list of resource objects")
						}
						urnAttr := dep.GetAttr("urn")
						if urnAttr.IsNull() || !urnAttr.IsKnown() || urnAttr.Type() != cty.String {
							return cty.NilVal, errors.New(
								"invoke options dependsOn must be a list of resource objects with known urn properties")
						}
						dependsOn = append(dependsOn, resource.URN(urnAttr.AsString()))
					}
				}

				provider := options.GetAttr("provider")
				if !provider.IsNull() && provider.IsKnown() {
					if !provider.Type().IsObjectType() {
						return cty.NilVal, errors.New("invoke options provider must be a resource object")
					}
					urnAttr := provider.GetAttr("urn")
					if urnAttr.IsNull() || !urnAttr.IsKnown() || urnAttr.Type() != cty.String {
						return cty.NilVal, errors.New("invoke options provider must be a resource object with known urn property")
					}
					idAttr := provider.GetAttr("id")
					if idAttr.IsNull() || !idAttr.IsKnown() || idAttr.Type() != cty.String {
						return cty.NilVal, errors.New("invoke options provider must be a resource object with known id property")
					}
					request.Provider = fmt.Sprintf("%s::%s", urnAttr.AsString(), idAttr.AsString())
				}
			}

			resp, err := i.monitor.Invoke(context.TODO(), request)
			if err != nil {
				return cty.NilVal, fmt.Errorf("invoke engine: %w", err)
			}
			if len(resp.Failures) > 0 {
				var buf strings.Builder
				buf.WriteString("invoke failed with the following errors:\n")
				for _, failure := range resp.Failures {
					buf.WriteString(fmt.Sprintf("- %s\n", failure))
				}
				return cty.NilVal, errors.New(buf.String())
			}

			resultPM, err := plugin.UnmarshalProperties(resp.GetReturn(), marshalOpts)
			if err != nil {
				return cty.NilVal, fmt.Errorf("unmarshal invoke result: %w", err)
			}
			// If this is a scalar invoke pull off the one property in the map
			var resultPV resource.PropertyValue
			if _, ok := fun.ReturnType.(*schema.ObjectType); ok {
				resultPV = resource.NewProperty(resultPM)
			} else {
				if len(resultPM) != 1 {
					return cty.NilVal, fmt.Errorf("expected scalar invoke result to have exactly one property, got %d", len(resultPM))
				}
				for _, v := range resultPM {
					resultPV = v
				}
			}
			if len(dependsOn) > 0 {
				resultPV = resource.NewProperty(resource.Output{
					Element:      resultPV,
					Known:        true,
					Dependencies: dependsOn,
				})
			}
			return propertyValueToCty(resultPV)
		},
	})

	callFn := function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "self",
				Type: cty.DynamicPseudoType,
			},
			{
				Name: "token",
				Type: cty.String,
			},
			{
				Name: "args",
				Type: cty.DynamicPseudoType,
			},
		},
		Type: function.StaticReturnType(cty.DynamicPseudoType),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			if len(args) < 3 {
				return cty.NilVal, errors.New("call requires a self, token, and arguments")
			}
			if len(args) > 4 {
				return cty.NilVal, errors.New("call accepts at most three arguments: self, token, arguments, and options")
			}
			if args[1].Type() != cty.String {
				return cty.NilVal, errors.New("call token must be a string")
			}
			method := args[1].AsString()

			// Given the method name we need to find the full token to pass to Call, that means looking up the resource
			// for `self` checking its methods, finding the one that matches the method name and then finding _that_
			// function object.

			if args[0].IsNull() || !args[0].IsKnown() || !args[0].Type().IsObjectType() {
				return cty.NilVal, errors.New("call self must be a known resource type")
			}
			self := args[0].AsValueMap()

			typ, has := self["__type"]
			if !has || typ.IsNull() || !typ.IsKnown() || typ.Type() != cty.String {
				return cty.NilVal, errors.New("call self must have a known __type property of type string")
			}

			typeStr := typ.AsString()
			schemaResource, err := i.lookupResource(context.TODO(), typeStr)
			if err != nil {
				return cty.NilVal, fmt.Errorf("lookup resource schema for token %s: %w", typeStr, err)
			}

			var fun *schema.Function
			for _, methodSchema := range schemaResource.Methods {
				if methodSchema.Name == method {
					fun = methodSchema.Function
					break
				}
			}
			if fun == nil {
				return cty.NilVal, fmt.Errorf("resource type %s does not have method %s", typeStr, method)
			}

			argsPV, err := ctyToPropertyValue(args[2])
			if err != nil {
				return cty.NilVal, fmt.Errorf("invalid invoke arguments: %w", err)
			}
			argsPM := argsPV.ObjectValue()

			urnVal, ok := self["urn"]
			if !ok || urnVal.IsNull() || !urnVal.IsKnown() || urnVal.Type() != cty.String {
				return cty.NilVal, errors.New("call self must have a known urn property of type string")
			}
			id, ok := self["id"]
			if !ok {
				return cty.NilVal, errors.New("call self must have an id property of type string")
			}

			urn := urnVal.AsString()
			argsPM["__self__"] = resource.NewProperty(resource.ResourceReference{
				URN: resource.URN(urn),
				ID:  resource.NewProperty(id.AsString()),
			})

			marshalOpts := plugin.MarshalOptions{
				KeepUnknowns:     true,
				KeepSecrets:      true,
				KeepResources:    true,
				KeepOutputValues: true,
			}
			obj, err := plugin.MarshalProperties(argsPM, marshalOpts)
			if err != nil {
				return cty.NilVal, fmt.Errorf("marshal invoke arguments: %w", err)
			}

			request := &pulumirpc.ResourceCallRequest{
				Tok:  fun.Token,
				Args: obj,
			}

			var dependsOn []resource.URN
			if len(args) == 4 && !args[3].IsNull() {
				options := args[3]

				deps := options.GetAttr("dependsOn")
				if !deps.IsNull() && deps.IsKnown() {
					if !deps.Type().IsListType() {
						return cty.NilVal, errors.New("invoke options dependsOn must be a list of strings")
					}
					for it := deps.ElementIterator(); it.Next(); {
						_, dep := it.Element()
						if dep.IsNull() || !dep.IsKnown() {
							continue
						}
						// dependencies should be resource objects that will have a urn property
						if !dep.Type().IsObjectType() {
							return cty.NilVal, errors.New("invoke options dependsOn must be a list of resource objects")
						}
						urnAttr := dep.GetAttr("urn")
						if urnAttr.IsNull() || !urnAttr.IsKnown() || urnAttr.Type() != cty.String {
							return cty.NilVal, errors.New(
								"invoke options dependsOn must be a list of resource objects with known urn properties")
						}
						dependsOn = append(dependsOn, resource.URN(urnAttr.AsString()))
					}
				}
			}

			resp, err := i.monitor.Call(context.TODO(), request)
			if err != nil {
				return cty.NilVal, fmt.Errorf("invoke engine: %w", err)
			}
			if len(resp.Failures) > 0 {
				var buf strings.Builder
				buf.WriteString("invoke failed with the following errors:\n")
				for _, failure := range resp.Failures {
					buf.WriteString(fmt.Sprintf("- %s\n", failure))
				}
				return cty.NilVal, errors.New(buf.String())
			}

			resultPM, err := plugin.UnmarshalProperties(resp.GetReturn(), marshalOpts)
			if err != nil {
				return cty.NilVal, fmt.Errorf("unmarshal invoke result: %w", err)
			}
			resultPV := resource.NewProperty(resultPM)
			if len(dependsOn) > 0 {
				resultPV = resource.NewProperty(resource.Output{
					Element:      resultPV,
					Known:        true,
					Dependencies: dependsOn,
				})
			}
			return propertyValueToCty(resultPV)
		},
	})

	fileAssetFn := function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "path",
				Type: cty.String,
			},
		},
		Type: function.StaticReturnType(assetType),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			if len(args) != 1 {
				return cty.NilVal, errors.New("fileAsset requires a path argument")
			}
			if args[0].Type() != cty.String {
				return cty.NilVal, errors.New("fileAsset path must be a string")
			}
			path := args[0].AsString()
			a, err := asset.FromPathWithWD(path, i.info.WorkingDir)
			if err != nil {
				return cty.NilVal, fmt.Errorf("creating file asset: %w", err)
			}
			return propertyValueToCty(resource.NewProperty(a))
		},
	})

	fileArchiveFn := function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "path",
				Type: cty.String,
			},
		},
		Type: function.StaticReturnType(archiveType),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			if len(args) != 1 {
				return cty.NilVal, errors.New("fileArchive requires a path argument")
			}
			if args[0].Type() != cty.String {
				return cty.NilVal, errors.New("fileArchive path must be a string")
			}
			path := args[0].AsString()
			a, err := archive.FromPathWithWD(path, i.info.WorkingDir)
			if err != nil {
				return cty.NilVal, fmt.Errorf("creating file archive: %w", err)
			}
			return propertyValueToCty(resource.NewProperty(a))
		},
	})

	assetArchiveFn := function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "assets",
				Type: cty.DynamicPseudoType,
			},
		},
		Type: function.StaticReturnType(archiveType),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			if len(args) != 1 {
				return cty.NilVal, errors.New("assertArchive requires an assets argument")
			}
			if !args[0].Type().IsMapType() && !args[0].Type().IsObjectType() {
				return cty.NilVal, errors.New("assetArchive assets must be a map or object")
			}

			assets := map[string]any{}
			for k, v := range args[0].AsValueMap() {
				assets[k] = v.EncapsulatedValue()
			}

			a, err := archive.FromAssetsWithWD(assets, i.info.WorkingDir)
			if err != nil {
				return cty.NilVal, fmt.Errorf("creating archive from assets: %w", err)
			}
			return propertyValueToCty(resource.NewProperty(a))
		},
	})

	stringAssetFn := function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "text",
				Type: cty.String,
			},
		},
		Type: function.StaticReturnType(assetType),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			if len(args) != 1 {
				return cty.NilVal, errors.New("stringAsset requires a text argument")
			}
			if args[0].Type() != cty.String {
				return cty.NilVal, errors.New("text must be a string")
			}
			text := args[0].AsString()
			a, err := asset.FromText(text)
			if err != nil {
				return cty.NilVal, fmt.Errorf("creating string asset: %w", err)
			}
			return propertyValueToCty(resource.NewProperty(a))
		},
	})

	remoteAssetFn := function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "uri",
				Type: cty.String,
			},
		},
		Type: function.StaticReturnType(assetType),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			if len(args) != 1 {
				return cty.NilVal, errors.New("remoteAsset requires a uri argument")
			}
			if args[0].Type() != cty.String {
				return cty.NilVal, errors.New("remoteAsset uri must be a string")
			}
			uri := args[0].AsString()
			a, err := asset.FromURI(uri)
			if err != nil {
				return cty.NilVal, fmt.Errorf("creating remote asset: %w", err)
			}
			return propertyValueToCty(resource.NewProperty(a))
		},
	})

	convertFn := function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name:      "value",
				Type:      cty.DynamicPseudoType,
				AllowNull: true,
			},
		},
		Type: function.StaticReturnType(cty.DynamicPseudoType),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			if len(args) != 1 {
				return cty.NilVal, errors.New("convert requires a value argument")
			}
			return args[0], nil
		},
	})

	pulumiResourceTypeFn := function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "resource",
				Type: cty.DynamicPseudoType,
			},
		},
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			if len(args) != 1 {
				return cty.NilVal, errors.New("pulumiResourceType requires a resource argument")
			}
			if !args[0].Type().IsObjectType() {
				return cty.NilVal, errors.New("pulumiResourceType argument must be an object")
			}
			res := args[0]
			name := res.GetAttr("__type")
			return name, nil
		},
	})

	pulumiResourceNameFn := function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "resource",
				Type: cty.DynamicPseudoType,
			},
		},
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			if len(args) != 1 {
				return cty.NilVal, errors.New("pulumiResourceName requires a resource argument")
			}
			if !args[0].Type().IsObjectType() {
				return cty.NilVal, errors.New("pulumiResourceName argument must be an object")
			}
			res := args[0]
			name := res.GetAttr("__name")
			return name, nil
		},
	})

	splitFn := function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "separator",
				Type: cty.String,
			},
			{
				Name: "string",
				Type: cty.String,
			},
		},
		Type: function.StaticReturnType(cty.List(cty.String)),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			if len(args) != 2 {
				return cty.NilVal, errors.New("split requires a separator and string argument")
			}
			if args[0].Type() != cty.String || args[1].Type() != cty.String {
				return cty.NilVal, errors.New("split arguments must be strings")
			}
			sep := args[0].AsString()
			str := args[1].AsString()
			parts := strings.Split(str, sep)
			ctyParts := make([]cty.Value, len(parts))
			for i, part := range parts {
				ctyParts[i] = cty.StringVal(part)
			}
			return cty.ListVal(ctyParts), nil
		},
	})

	elementFn := function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "list",
				Type: cty.List(cty.DynamicPseudoType),
			},
			{
				Name: "index",
				Type: cty.Number,
			},
		},
		Type: func(args []cty.Value) (cty.Type, error) {
			return args[0].Index(args[1]).Type(), nil
		},
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			if len(args) != 2 {
				return cty.NilVal, errors.New("element requires a list and index argument")
			}
			return args[0].Index(args[1]), nil
		},
	})

	joinFn := function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "separator",
				Type: cty.String,
			},
			{
				Name: "strings",
				Type: cty.List(cty.String),
			},
		},
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			parts := make([]string, 0, args[1].LengthInt())
			for _, part := range args[1].AsValueSlice() {
				parts = append(parts, part.AsString())
			}
			sep := args[0].AsString()
			return cty.StringVal(strings.Join(parts, sep)), nil
		},
	})

	lengthFn := function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "list",
				Type: cty.List(cty.DynamicPseudoType),
			},
		},
		Type: function.StaticReturnType(cty.Number),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			return cty.NumberIntVal(int64(args[0].LengthInt())), nil
		},
	})

	singleOrNoneFn := function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "list",
				Type: cty.List(cty.DynamicPseudoType),
			},
		},
		Type: func(args []cty.Value) (cty.Type, error) {
			if args[0].LengthInt() == 0 {
				return cty.DynamicPseudoType, nil
			}
			return args[0].Index(cty.NumberIntVal(0)).Type(), nil
		},
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			if args[0].LengthInt() == 0 {
				return cty.NilVal, nil
			}
			if args[0].LengthInt() == 1 {
				return args[0].Index(cty.NumberIntVal(0)), nil
			}
			return cty.NilVal, fmt.Errorf("expected list to have at most one element, got %d", args[0].LengthInt())
		},
	})

	entriesFn := function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "collection",
				Type: cty.DynamicPseudoType,
			},
		},
		Type: function.StaticReturnType(cty.List(cty.Object(map[string]cty.Type{
			"key":   cty.String,
			"value": cty.DynamicPseudoType,
		}))),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			if len(args) != 1 {
				return cty.NilVal, errors.New("entries requires a collection argument")
			}
			if !args[0].Type().IsMapType() && !args[0].Type().IsObjectType() {
				return cty.NilVal, fmt.Errorf("entries argument must be a collection, was %s", args[0].Type().FriendlyName())
			}
			obj := args[0]
			var entries []cty.Value
			for k, v := range obj.AsValueMap() {
				entry := cty.ObjectVal(map[string]cty.Value{
					"key":   cty.StringVal(k),
					"value": v,
				})
				entries = append(entries, entry)
			}
			return cty.ListVal(entries), nil
		},
	})

	lookupFn := function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "collection",
				Type: cty.DynamicPseudoType,
			},
			{
				Name: "key",
				Type: cty.String,
			},
			{
				Name: "default",
				Type: cty.DynamicPseudoType,
			},
		},
		Type: func(args []cty.Value) (cty.Type, error) {
			return args[2].Type(), nil
		},
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			if len(args) != 3 {
				return cty.NilVal, errors.New("lookup requires a collection, key, and default argument")
			}
			if args[1].Type() != cty.String {
				return cty.NilVal, errors.New("lookup key argument must be a string")
			}
			collection := args[0]
			key := args[1].AsString()
			defaultVal := args[2]

			if collection.Type().IsMapType() || collection.Type().IsObjectType() {
				val := collection.AsValueMap()
				if elem, ok := val[key]; ok {
					return elem, nil
				} else {
					return defaultVal, nil
				}
			}

			return cty.NilVal, fmt.Errorf(
				"lookup collection argument must be a map or object, was %s",
				collection.Type().FriendlyName())
		},
	})

	toBase64Fn := function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "data",
				Type: cty.String,
			},
		},
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			if len(args) != 1 {
				return cty.NilVal, errors.New("toBase64 requires a data argument")
			}
			if args[0].Type() != cty.String {
				return cty.NilVal, errors.New("toBase64 data argument must be a string")
			}
			data := args[0].AsString()
			encoded := base64.StdEncoding.EncodeToString([]byte(data))
			return cty.StringVal(encoded), nil
		},
	})

	fromBase64Fn := function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "data",
				Type: cty.String,
			},
		},
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			if len(args) != 1 {
				return cty.NilVal, errors.New("fromBase64 requires a data argument")
			}
			if args[0].Type() != cty.String {
				return cty.NilVal, errors.New("fromBase64 data argument must be a string")
			}
			data := args[0].AsString()
			decodedBytes, err := base64.StdEncoding.DecodeString(data)
			if err != nil {
				return cty.NilVal, fmt.Errorf("invalid base64 data: %w", err)
			}
			return cty.StringVal(string(decodedBytes)), nil
		},
	})

	return map[string]function.Function{
		"cwd":                stringFn(i.info.WorkingDir),
		"rootDirectory":      stringFn(i.info.RootDirectory),
		"project":            stringFn(i.info.Project),
		"stack":              stringFn(i.info.Stack),
		"organization":       stringFn(i.info.Organization),
		"secret":             secretFn,
		"unsecret":           unsecretFn,
		"try":                tryFn,
		"can":                canFn,
		"getOutput":          getOutputFn,
		"invoke":             invokeFn,
		"call":               callFn,
		"fileAsset":          fileAssetFn,
		"fileArchive":        fileArchiveFn,
		"assetArchive":       assetArchiveFn,
		"stringAsset":        stringAssetFn,
		"remoteAsset":        remoteAssetFn,
		"__convert":          convertFn,
		"pulumiResourceType": pulumiResourceTypeFn,
		"pulumiResourceName": pulumiResourceNameFn,
		"split":              splitFn,
		"element":            elementFn,
		"join":               joinFn,
		"length":             lengthFn,
		"singleOrNone":       singleOrNoneFn,
		"entries":            entriesFn,
		"lookup":             lookupFn,
		"toBase64":           toBase64Fn,
		"fromBase64":         fromBase64Fn,
	}
}
