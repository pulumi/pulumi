// Copyright 2016-2026, Pulumi Corporation.
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
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/ext/customdecode"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/archive"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/asset"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

var invokeOptionsType = cty.ObjectWithOptionalAttrs(map[string]cty.Type{
	"version":   cty.String,
	"dependsOn": cty.List(cty.DynamicPseudoType),
}, []string{
	"version",
	"dependsOn",
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
	DryRun         bool
	Parallel       int32
}

type Interpreter struct {
	program *pcl.Program
	info    RunInfo

	monitor pulumirpc.ResourceMonitorClient
	engine  pulumirpc.EngineClient

	evalContext *hcl.EvalContext
	variables   map[string]resource.PropertyValue
	stackURN    string
}

func NewInterpreter(program *pcl.Program, info RunInfo) *Interpreter {
	return &Interpreter{program: program, info: info}
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

	i.variables = map[string]resource.PropertyValue{}
	i.evalContext = &hcl.EvalContext{
		Variables: map[string]cty.Value{},
		Functions: i.builtinFunctions(),
	}

	if diags := i.bindConfigVariables(); diags.HasErrors() {
		return diags
	}

	if err := i.enforceRequiredVersion(ctx); err != nil {
		return err
	}

	if err := i.registerStack(ctx); err != nil {
		return err
	}

	outputs := resource.PropertyMap{}
	for _, node := range i.program.Nodes {
		switch node := node.(type) {
		case *pcl.ConfigVariable:
			// handled above
		case *pcl.PulumiBlock:
			// handled above
		case *pcl.LocalVariable:
			value, diags := i.evalExpression(node.Definition.Value)
			if diags.HasErrors() {
				return diags
			}
			if err := i.setVariable(node.Name(), value); err != nil {
				return err
			}
		case *pcl.Resource:
			if err := i.registerResource(ctx, node); err != nil {
				return err
			}
		case *pcl.OutputVariable:
			value, diags := i.evalExpression(node.Value)
			if diags.HasErrors() {
				return diags
			}
			outputs[resource.PropertyKey(node.LogicalName())] = value
		}
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

func (i *Interpreter) evalExpression(expr model.Expression) (resource.PropertyValue, hcl.Diagnostics) {
	value, diags := expr.Evaluate(i.evalContext)
	if diags.HasErrors() {
		return resource.PropertyValue{}, diags
	}
	pv, err := ctyToPropertyValue(value)
	if err != nil {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  err.Error(),
		})
		return resource.PropertyValue{}, diags
	}
	return pv, diags
}

func (i *Interpreter) bindConfigVariables() hcl.Diagnostics {
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
				value, diags := i.evalExpression(cfg.DefaultValue)
				diagnostics = append(diagnostics, diags...)
				if _, isSecret := secretKeys[key]; isSecret {
					value = resource.MakeSecret(value)
				}
				if !diags.HasErrors() {
					if err := i.setVariable(cfg.Name(), value); err != nil {
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
			if _, isSecret := secretKeys[key]; isSecret {
				value = resource.MakeSecret(value)
			}
			if err := i.setVariable(cfg.Name(), value); err != nil {
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

	value, diags := i.evalExpression(required)
	if diags.HasErrors() {
		return diags
	}
	if !value.IsString() {
		return fmt.Errorf("requiredVersion must be a string")
	}
	if i.engine == nil {
		return fmt.Errorf("engine client not available to validate requiredVersion")
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
			KeepUnknowns:     true,
			KeepSecrets:      true,
			KeepResources:    true,
			KeepOutputValues: true,
		}
		objectValue, err := plugin.UnmarshalProperties(resp.Object, marshalOpts)
		if err == nil {
			contract.IgnoreError(i.setVariable("pulumi", resource.NewProperty(objectValue)))
		}
	}
	i.stackURN = resp.GetUrn()
	return nil
}

func (i *Interpreter) registerResource(ctx context.Context, res *pcl.Resource) error {
	inputs := resource.PropertyMap{}
	for _, attr := range res.Inputs {
		val, diags := i.evalExpression(attr.Value)
		if diags.HasErrors() {
			return diags
		}
		inputs[resource.PropertyKey(attr.Name)] = val
	}

	marshalOpts := plugin.MarshalOptions{
		KeepUnknowns:     true,
		KeepSecrets:      true,
		KeepResources:    true,
		KeepOutputValues: true,
	}
	obj, err := plugin.MarshalProperties(inputs, marshalOpts)
	if err != nil {
		return err
	}

	custom := true
	if res.Schema != nil {
		custom = !res.Schema.IsComponent
	}

	components := strings.Split(res.Token, ":")
	contract.Assertf(len(components) == 3, "invalid token format: %s", res.Token)
	if components[1] == "" {
		components[1] = "index"
	}
	token := fmt.Sprintf("%s:%s:%s", components[0], components[1], components[2])

	request := &pulumirpc.RegisterResourceRequest{
		Type:   token,
		Name:   res.LogicalName(),
		Custom: custom,
		Object: obj,
	}

	if res.Options != nil {
		if res.Options.ReplaceOnChanges != nil {
			replaceOnChanges, diags := i.evalExpression(res.Options.ReplaceOnChanges)
			if diags.HasErrors() {
				return diags
			}
			rocopt := []string{}
			if !replaceOnChanges.IsArray() {
				return fmt.Errorf("replaceOnChanges must be an array of strings")
			}
			for _, v := range replaceOnChanges.ArrayValue() {
				if v.IsNull() || v.IsComputed() {
					continue
				}
				if !v.IsString() {
					return fmt.Errorf("replaceOnChanges must be an array of strings")
				}
				rocopt = append(rocopt, v.StringValue())
			}
			request.ReplaceOnChanges = rocopt
		}
		if res.Options.ReplacementTrigger != nil {
			replacement, diags := i.evalExpression(res.Options.ReplacementTrigger)
			if diags.HasErrors() {
				return diags
			}
			request.ReplacementTrigger, err = plugin.MarshalPropertyValue(
				"replacementTrigger", replacement, marshalOpts)
			if err != nil {
				return err
			}
		}
	}

	resp, err := i.monitor.RegisterResource(ctx, request)
	if err != nil {
		return err
	}
	if resp.GetResult() != pulumirpc.Result_SUCCESS {
		return fmt.Errorf("resource registration failed: %s", resp.GetResult())
	}

	outputs, err := plugin.UnmarshalProperties(resp.Object, marshalOpts)
	if err != nil {
		return err
	}

	outputs["id"] = resource.NewProperty(resp.GetId())
	outputs["urn"] = resource.NewProperty(resp.GetUrn())

	if err := i.setVariable(res.Name(), resource.NewProperty(outputs)); err != nil {
		return err
	}
	return nil
}

func (i *Interpreter) registerStackOutputs(ctx context.Context, outputs resource.PropertyMap) error {
	if i.stackURN == "" {
		return errors.New("missing stack URN")
	}
	marshalOpts := plugin.MarshalOptions{
		KeepUnknowns:     true,
		KeepSecrets:      true,
		KeepResources:    true,
		KeepOutputValues: true,
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
				Name: "value",
				Type: cty.DynamicPseudoType,
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
				return cty.NilVal, fmt.Errorf("secret requires a value")
			}
			return args[0].Mark(secretMark{}), nil
		},
	})

	unsecretFn := function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "value",
				Type: cty.DynamicPseudoType,
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
				return cty.NilVal, fmt.Errorf("unsecret requires a value")
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
				return cty.NilVal, fmt.Errorf("can requires an expression")
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
				return cty.NilVal, fmt.Errorf("getOutput requires a stack reference and output name")
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
			{
				Name:      "options",
				Type:      invokeOptionsType,
				AllowNull: true,
			},
		},
		Type: function.StaticReturnType(cty.DynamicPseudoType),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			if len(args) < 2 {
				return cty.NilVal, fmt.Errorf("invoke requires a token and arguments")
			}
			if len(args) > 3 {
				return cty.NilVal, fmt.Errorf("invoke accepts at most three arguments: token, arguments, and options")
			}
			if args[0].Type() != cty.String {
				return cty.NilVal, fmt.Errorf("invoke token must be a string")
			}
			token := args[0].AsString()
			components := strings.Split(token, ":")
			contract.Assertf(len(components) == 3, "invalid token format: %s", token)
			if components[1] == "" {
				components[1] = "index"
			}
			token = fmt.Sprintf("%s:%s:%s", components[0], components[1], components[2])

			argsPV, err := ctyToPropertyValue(args[1])
			if err != nil {
				return cty.NilVal, fmt.Errorf("invalid invoke arguments: %w", err)
			}

			marshalOpts := plugin.MarshalOptions{
				KeepUnknowns:     true,
				KeepSecrets:      true,
				KeepResources:    true,
				KeepOutputValues: true,
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
						return cty.NilVal, fmt.Errorf("invoke options dependsOn must be a list of strings")
					}
					for it := deps.ElementIterator(); it.Next(); {
						_, dep := it.Element()
						if dep.IsNull() || !dep.IsKnown() {
							continue
						}
						// dependencies should be resource objects that will have a urn property
						if !dep.Type().IsObjectType() {
							return cty.NilVal, fmt.Errorf("invoke options dependsOn must be a list of resource objects")
						}
						urnAttr := dep.GetAttr("urn")
						if urnAttr.IsNull() || !urnAttr.IsKnown() || urnAttr.Type() != cty.String {
							return cty.NilVal, fmt.Errorf("invoke options dependsOn must be a list of resource objects with known urn properties")
						}
						dependsOn = append(dependsOn, resource.URN(urnAttr.AsString()))
					}
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
			resultPV := resource.NewProperty(resultPM)
			if len(dependsOn) > 0 {
				resultPV = resource.NewOutputProperty(resource.Output{
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
			return cty.NilVal, fmt.Errorf("call is not supported yet")
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
				return cty.NilVal, fmt.Errorf("fileAsset requires a path argument")
			}
			if args[0].Type() != cty.String {
				return cty.NilVal, fmt.Errorf("fileAsset path must be a string")
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
				return cty.NilVal, fmt.Errorf("fileArchive requires a path argument")
			}
			if args[0].Type() != cty.String {
				return cty.NilVal, fmt.Errorf("fileArchive path must be a string")
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
				return cty.NilVal, fmt.Errorf("assertArchive requires an assets argument")
			}
			if !args[0].Type().IsMapType() && !args[0].Type().IsObjectType() {
				return cty.NilVal, fmt.Errorf("assetArchive assets must be a map or object")
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
				return cty.NilVal, fmt.Errorf("stringAsset requires a text argument")
			}
			if args[0].Type() != cty.String {
				return cty.NilVal, fmt.Errorf("text must be a string")
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
				return cty.NilVal, fmt.Errorf("remoteAsset requires a uri argument")
			}
			if args[0].Type() != cty.String {
				return cty.NilVal, fmt.Errorf("remoteAsset uri must be a string")
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
				return cty.NilVal, fmt.Errorf("convert requires a value argument")
			}
			return args[0], nil
		},
	})

	return map[string]function.Function{
		"cwd":           stringFn(i.info.WorkingDir),
		"rootDirectory": stringFn(i.info.RootDirectory),
		"project":       stringFn(i.info.Project),
		"stack":         stringFn(i.info.Stack),
		"organization":  stringFn(i.info.Organization),
		"secret":        secretFn,
		"unsecret":      unsecretFn,
		"try":           tryFn,
		"can":           canFn,
		"getOutput":     getOutputFn,
		"invoke":        invokeFn,
		"call":          callFn,
		"fileAsset":     fileAssetFn,
		"fileArchive":   fileArchiveFn,
		"assetArchive":  assetArchiveFn,
		"stringAsset":   stringAssetFn,
		"remoteAsset":   remoteAssetFn,
		"__convert":     convertFn,
	}
}

func (i *Interpreter) setVariable(name string, value resource.PropertyValue) error {
	i.variables[name] = value
	ctyValue, err := propertyValueToCty(value)
	if err != nil {
		return err
	}
	i.evalContext.Variables[name] = ctyValue
	return nil
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

func tryExpressions(args []cty.Value) (cty.Value, error) {
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
		return propertyValueToCty(pv)
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
