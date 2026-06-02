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
	"crypto/sha1" //nolint:gosec // we don't need a strong cryptographic primitive
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/ext/customdecode"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/archive"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/asset"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	"github.com/zclconf/go-cty/cty/function/stdlib"
	ctyjson "github.com/zclconf/go-cty/cty/json"
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

func tryExpressions(
	args []cty.Value,
	getResource func(context.Context, resource.ResourceReference) (resource.PropertyMap, error),
) (cty.Value, error) {
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
		return propertyValueToCty(context.TODO(), getResource, pv)
	}

	var buf strings.Builder
	buf.WriteString("no expression succeeded:\n")
	for _, diag := range diags {
		if diag.Subject != nil {
			fmt.Fprintf(&buf, "- %s (at %s)\n  %s\n", diag.Summary, diag.Subject, diag.Detail)
		} else {
			fmt.Fprintf(&buf, "- %s\n  %s\n", diag.Summary, diag.Detail)
		}
	}
	buf.WriteString("\nAt least one expression must produce a successful result")
	return cty.NilVal, errors.New(buf.String())
}

func (ectx *EvalContext) builtinFunctions() map[string]function.Function {
	// If errorName is set and value is empty, the function will return an error with the given name. This is used for
	// functions that are only supported in some contexts, like rootDirectory not always being available in `pulumi do`.
	literalStringFn := func(value, errorName string) function.Function {
		return function.New(&function.Spec{
			Params: []function.Parameter{},
			Type:   function.StaticReturnType(cty.String),
			Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
				if errorName != "" && value == "" {
					return cty.NilVal, fmt.Errorf("%s is not supported", errorName)
				}
				return cty.StringVal(value), nil
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
			v, err := tryExpressions(args, ectx.getResource)
			if err != nil {
				return cty.NilType, err
			}
			return v.Type(), nil
		},
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			return tryExpressions(args, ectx.getResource)
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
				return propertyValueToCty(context.TODO(), ectx.getResource, resource.MakeSecret(resource.NewProperty(true)))
			}
			return cty.True, nil
		},
	})

	getOutputFn := function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name:        "stackReference",
				Type:        cty.DynamicPseudoType,
				AllowMarked: true,
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
			stackRefPV, _ = unwrapOutputs(stackRefPV)
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
			return propertyValueToCty(context.TODO(), ectx.getResource, outputPV)
		},
	})

	invokeFn := function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "token",
				Type: cty.String,
			},
			{
				Name:        "args",
				Type:        cty.DynamicPseudoType,
				AllowMarked: true,
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

			fun, err := ectx.lookupFunction(context.TODO(), token)
			if err != nil {
				return cty.NilVal, fmt.Errorf("lookup function for token %s: %w", token, err)
			}
			// Use the canonical schema token (the alias lookup in Get may have resolved a
			// normalized form like "pkg:index:name" to the source-form "pkg:index_name:name").
			token = fun.Token

			argsPV, err := ctyToPropertyValue(args[1])
			if err != nil {
				return cty.NilVal, fmt.Errorf("invalid invoke arguments: %w", err)
			}
			argsPV, dependsOn := unwrapOutputs(argsPV)
			if fun.Inputs != nil {
				args, err := applySchemaInputs(argsPV.ObjectValue(), fun.Inputs.Properties)
				if err != nil {
					return cty.NilVal, fmt.Errorf("convert invoke arguments: %w", err)
				}
				argsPV = resource.NewProperty(args)
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

			resp, err := ectx.invoke(context.TODO(), request)
			if err != nil {
				return cty.NilVal, fmt.Errorf("invoke engine: %w", err)
			}
			if len(resp.Failures) > 0 {
				var buf strings.Builder
				buf.WriteString("invoke failed with the following errors:\n")
				for _, failure := range resp.Failures {
					fmt.Fprintf(&buf, "- %s\n", failure)
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
			return propertyValueToCty(context.TODO(), ectx.getResource, resultPV)
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
				Name:        "args",
				Type:        cty.DynamicPseudoType,
				AllowMarked: true,
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
			schemaResource, err := ectx.lookupResource(context.TODO(), typeStr)
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
			argsPV, _ = unwrapOutputs(argsPV)
			argsPM := argsPV.ObjectValue()
			if fun.Inputs != nil {
				argsPM, err = applySchemaInputs(argsPM, fun.Inputs.Properties)
				if err != nil {
					return cty.NilVal, fmt.Errorf("convert call arguments: %w", err)
				}
			}

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

			resp, err := ectx.call(context.TODO(), request)
			if err != nil {
				return cty.NilVal, fmt.Errorf("invoke engine: %w", err)
			}
			if len(resp.Failures) > 0 {
				var buf strings.Builder
				buf.WriteString("invoke failed with the following errors:\n")
				for _, failure := range resp.Failures {
					fmt.Fprintf(&buf, "- %s\n", failure)
				}
				return cty.NilVal, errors.New(buf.String())
			}

			resultPM, err := plugin.UnmarshalProperties(resp.GetReturn(), marshalOpts)
			if err != nil {
				return cty.NilVal, fmt.Errorf("unmarshal invoke result: %w", err)
			}
			// Methods declared with ReturnTypePlain but no object return type carry the single value in a
			// property map with exactly one entry, whose key may be any name. Unwrap it so callers get the
			// value directly.
			var resultPV resource.PropertyValue
			if fun.ReturnTypePlain {
				if _, isObject := fun.ReturnType.(*schema.ObjectType); !isObject {
					if len(resultPM) != 1 {
						return cty.NilVal, fmt.Errorf(
							"invoke %q: expected a single return value, got %d", fun.Token, len(resultPM))
					}
					for _, v := range resultPM {
						resultPV = v
					}
				} else {
					resultPV = resource.NewProperty(resultPM)
				}
			} else {
				resultPV = resource.NewProperty(resultPM)
			}
			if len(dependsOn) > 0 {
				resultPV = resource.NewProperty(resource.Output{
					Element:      resultPV,
					Known:        true,
					Dependencies: dependsOn,
				})
			}
			return propertyValueToCty(context.TODO(), ectx.getResource, resultPV)
		},
	})

	// resourceExists reports whether a resource of the given type with the given ID exists. The name parameter has no
	// runtime effect--a resource being checked for existence has no logical identity--but is accepted to match the
	// signature used by the code generators. If the ID is unknown (e.g. during preview) the result is unknown.
	existsResourceFn := function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "type",
				Type: cty.String,
			},
			{
				Name: "name",
				Type: cty.String,
			},
			{
				Name:        "id",
				Type:        cty.String,
				AllowMarked: true,
			},
		},
		Type: function.StaticReturnType(cty.Bool),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			// The ID is typically an output of the resource being checked, so it carries secret and dependency
			// marks. Strip them before reading the underlying string.
			id, _ := args[2].Unmark()
			// During preview the ID of a resource that has not yet been created is unknown; the runtime represents
			// this as an empty string. We can't check existence without an ID, so the result is unknown.
			if !id.IsKnown() || id.AsString() == "" {
				return cty.UnknownVal(cty.Bool), nil
			}
			resp, err := ectx.existsResource(context.TODO(), &pulumirpc.ExistsResourceRequest{
				Type: args[0].AsString(),
				Id:   id.AsString(),
			})
			if err != nil {
				return cty.NilVal, fmt.Errorf("resourceExists: %w", err)
			}
			return cty.BoolVal(resp.GetExists()), nil
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
			a, err := asset.FromPathWithWD(path, ectx.workingDirectory)
			if err != nil {
				return cty.NilVal, fmt.Errorf("creating file asset: %w", err)
			}
			return propertyValueToCty(context.TODO(), ectx.getResource, resource.NewProperty(a))
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
			a, err := archive.FromPathWithWD(path, ectx.workingDirectory)
			if err != nil {
				return cty.NilVal, fmt.Errorf("creating file archive: %w", err)
			}
			return propertyValueToCty(context.TODO(), ectx.getResource, resource.NewProperty(a))
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

			a, err := archive.FromAssetsWithWD(assets, ectx.workingDirectory)
			if err != nil {
				return cty.NilVal, fmt.Errorf("creating archive from assets: %w", err)
			}
			return propertyValueToCty(context.TODO(), ectx.getResource, resource.NewProperty(a))
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
			return propertyValueToCty(context.TODO(), ectx.getResource, resource.NewProperty(a))
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
			return propertyValueToCty(context.TODO(), ectx.getResource, resource.NewProperty(a))
		},
	})

	remoteArchiveFn := function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "uri",
				Type: cty.String,
			},
		},
		Type: function.StaticReturnType(archiveType),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			if len(args) != 1 {
				return cty.NilVal, errors.New("remoteArchive requires a uri argument")
			}
			if args[0].Type() != cty.String {
				return cty.NilVal, errors.New("remoteArchive uri must be a string")
			}
			uri := args[0].AsString()
			a, err := archive.FromURI(uri)
			if err != nil {
				return cty.NilVal, fmt.Errorf("creating remote archive: %w", err)
			}
			return propertyValueToCty(context.TODO(), ectx.getResource, resource.NewProperty(a))
		},
	})

	convertFn := function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name:             "value",
				Type:             cty.DynamicPseudoType,
				AllowNull:        true,
				AllowMarked:      true,
				AllowUnknown:     true,
				AllowDynamicType: true,
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
			valueMap := obj.AsValueMap()
			keys := make([]string, 0, len(valueMap))
			for k := range valueMap {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			var entries []cty.Value
			for _, k := range keys {
				entry := cty.ObjectVal(map[string]cty.Value{
					"key":   cty.StringVal(k),
					"value": valueMap[k],
				})
				entries = append(entries, entry)
			}
			return cty.ListVal(entries), nil
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

	sha1Fn := function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "input",
				Type: cty.String,
			},
		},
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			if len(args) != 1 {
				return cty.NilVal, errors.New("sha1 requires an input argument")
			}
			if args[0].Type() != cty.String {
				return cty.NilVal, errors.New("sha1 input argument must be a string")
			}
			h := sha1.Sum([]byte(args[0].AsString())) //nolint:gosec // we don't need a strong cryptographic primitive
			return cty.StringVal(hex.EncodeToString(h[:])), nil
		},
	})

	toJSONFn := function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name:             "value",
				Type:             cty.DynamicPseudoType,
				AllowMarked:      true,
				AllowNull:        true,
				AllowDynamicType: true,
			},
		},
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			if len(args) != 1 {
				return cty.NilVal, errors.New("toJSON requires a value argument")
			}
			// UnmarkDeep strips marks from the value and all nested values, collecting them all.
			// We re-apply them to the resulting string so that e.g. a secret nested anywhere in
			// the input causes the JSON output to be secret too.
			val, marks := args[0].UnmarkDeep()
			if !val.IsWhollyKnown() {
				return cty.UnknownVal(cty.String).WithMarks(marks), nil
			}
			if val.IsNull() {
				return cty.StringVal("null").WithMarks(marks), nil
			}
			buf, err := ctyjson.Marshal(val, val.Type())
			if err != nil {
				return cty.NilVal, fmt.Errorf("toJSON: %w", err)
			}
			return cty.StringVal(string(buf)).WithMarks(marks), nil
		},
	})

	resolvePath := func(p string) string {
		if filepath.IsAbs(p) {
			return p
		}
		return filepath.Join(ectx.workingDirectory, p)
	}

	readFileFn := function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "path",
				Type: cty.String,
			},
		},
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			path := resolvePath(args[0].AsString())
			data, err := os.ReadFile(path)
			if err != nil {
				return cty.NilVal, fmt.Errorf("readFile: %w", err)
			}
			return cty.StringVal(string(data)), nil
		},
	})

	filebase64Fn := function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "path",
				Type: cty.String,
			},
		},
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			path := resolvePath(args[0].AsString())
			data, err := os.ReadFile(path)
			if err != nil {
				return cty.NilVal, fmt.Errorf("filebase64: %w", err)
			}
			return cty.StringVal(base64.StdEncoding.EncodeToString(data)), nil
		},
	})

	filebase64sha256Fn := function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "path",
				Type: cty.String,
			},
		},
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			path := resolvePath(args[0].AsString())
			data, err := os.ReadFile(path)
			if err != nil {
				return cty.NilVal, fmt.Errorf("filebase64sha256: %w", err)
			}
			hash := sha256.Sum256(data)
			return cty.StringVal(base64.StdEncoding.EncodeToString(hash[:])), nil
		},
	})

	lengthFunc := function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "value",
				Type: cty.DynamicPseudoType,
			},
		},
		Type: function.StaticReturnType(cty.Number),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			if args[0].Type() == cty.String {
				return stdlib.Strlen(args[0])
			}
			return stdlib.Length(args[0])
		},
	})

	return map[string]function.Function{
		"cwd":                literalStringFn(ectx.workingDirectory, ""),
		"rootDirectory":      literalStringFn(ectx.rootDirectory, "rootDirectory"),
		"project":            literalStringFn(ectx.project, "project"),
		"stack":              literalStringFn(ectx.stack, "stack"),
		"organization":       literalStringFn(ectx.organization, "organization"),
		"secret":             secretFn,
		"unsecret":           unsecretFn,
		"try":                tryFn,
		"can":                canFn,
		"getOutput":          getOutputFn,
		"invoke":             invokeFn,
		"call":               callFn,
		"resourceExists":     existsResourceFn,
		"fileAsset":          fileAssetFn,
		"fileArchive":        fileArchiveFn,
		"assetArchive":       assetArchiveFn,
		"stringAsset":        stringAssetFn,
		"remoteAsset":        remoteAssetFn,
		"remoteArchive":      remoteArchiveFn,
		"__convert":          convertFn,
		"pulumiResourceType": pulumiResourceTypeFn,
		"pulumiResourceName": pulumiResourceNameFn,
		"split":              stdlib.SplitFunc,
		"element":            stdlib.ElementFunc,
		"join":               stdlib.JoinFunc,
		"length":             lengthFunc,
		"singleOrNone":       singleOrNoneFn,
		"entries":            entriesFn,
		"lookup":             stdlib.LookupFunc,
		"toBase64":           toBase64Fn,
		"fromBase64":         fromBase64Fn,
		"toJSON":             toJSONFn,
		"sha1":               sha1Fn,
		"readFile":           readFileFn,
		"filebase64":         filebase64Fn,
		"filebase64sha256":   filebase64sha256Fn,
		"max": function.New(&function.Spec{
			VarParam: &function.Parameter{
				Name: "numbers",
				Type: cty.Number,
			},
			Type: function.StaticReturnType(cty.Number),
			Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
				if len(args) == 0 {
					return cty.NilVal, errors.New("max requires at least one argument")
				}
				result := args[0]
				for _, arg := range args[1:] {
					if arg.GreaterThan(result).True() {
						result = arg
					}
				}
				return result, nil
			},
		}),
		"min": function.New(&function.Spec{
			VarParam: &function.Parameter{
				Name: "numbers",
				Type: cty.Number,
			},
			Type: function.StaticReturnType(cty.Number),
			Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
				if len(args) == 0 {
					return cty.NilVal, errors.New("min requires at least one argument")
				}
				result := args[0]
				for _, arg := range args[1:] {
					if arg.LessThan(result).True() {
						result = arg
					}
				}
				return result, nil
			},
		}),
	}
}
