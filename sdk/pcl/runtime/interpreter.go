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
}

func NewInterpreter(program *pcl.Program, info RunInfo) *Interpreter {
	return &Interpreter{program: program, info: info}
}

type node struct{}

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

	if diags := i.bindConfigVariables(); diags.HasErrors() {
		return diags
	}

	if err := i.enforceRequiredVersion(ctx); err != nil {
		return err
	}

	if err := i.registerStack(ctx); err != nil {
		return err
	}

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
			dag.NewEdge(dagNodeB, dagNodeA)
		}
	}

	var outputsLock sync.Mutex
	outputs := resource.PropertyMap{}
	err = dag.Walk(ctx, func(ctx context.Context, node pcl.Node) error {
		switch node := node.(type) {
		case *pcl.ConfigVariable:
			// handled above
			return nil
		case *pcl.PulumiBlock:
			// handled above
			return nil
		case *pcl.LocalVariable:
			value, diags := i.evalExpression(node.Definition.Value)
			if diags.HasErrors() {
				return diags
			}
			if err := i.setVariable(node.Name(), value); err != nil {
				return fmt.Errorf("failed to set variable %s: %w", node.Name(), err)
			}
		case *pcl.Resource:
			if err := i.registerResource(ctx, node); err != nil {
				return fmt.Errorf("failed to register resource %s: %w", node.Name(), err)
			}
		case *pcl.OutputVariable:
			value, diags := i.evalExpression(node.Value)
			if diags.HasErrors() {
				return diags
			}
			outputsLock.Lock()
			defer outputsLock.Unlock()
			outputs[resource.PropertyKey(node.LogicalName())] = value
		default:
			return fmt.Errorf("unknown node type: %T", node)
		}
		return nil
	}, pdag.MaxProcs(int(i.info.Parallel)))
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

func (i *Interpreter) lookupResource(ctx context.Context, token string) (*schema.Resource, error) {
	components := strings.Split(token, ":")
	contract.Assertf(len(components) == 3, "invalid token format: %s", token)
	if components[1] == "" {
		components[1] = "index"
	}
	token = fmt.Sprintf("%s:%s:%s", components[0], components[1], components[2])
	pkgName := components[0]
	if components[0] == "pulumi" && components[1] == "providers" {
		pkgName = components[2]
	}

	// Fall back to just the package name and passed in version if we don't have a descriptor.
	descriptor := &schema.PackageDescriptor{
		Name: pkgName,
	}
	pkg, err := i.loader.LoadPackageReferenceV2(ctx, descriptor)
	if err != nil {
		return nil, fmt.Errorf("load package for token %s: %w", token, err)
	}
	if components[0] == "pulumi" && components[1] == "providers" {
		return pkg.Provider()
	}
	resources := pkg.Resources()
	schemaResource, ok, err := resources.Get(token)
	if err != nil {
		return nil, fmt.Errorf("get resource from package for token %s: %w", token, err)
	}
	if !ok {
		// Didn't find the resource via a direct lookup, we now need to iterate _all_ the resources and use TokenToModule to see if any of the match the
		// token we have.
		iter := resources.Range()
		for iter.Next() {
			resToken := iter.Token()
			// Canonicalize the resources token via TokenToModule
			mod := pkg.TokenToModule(resToken)
			components := strings.Split(resToken, ":")
			resToken = fmt.Sprintf("%s:%s:%s", components[0], mod, components[2])
			if token == resToken {
				token = iter.Token()
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

func (i *Interpreter) evalExpression(expr model.Expression) (resource.PropertyValue, hcl.Diagnostics) {
	i.evalLock.Lock()
	value, diags := expr.Evaluate(i.evalContext)
	i.evalLock.Unlock()
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
			KeepUnknowns:  true,
			KeepSecrets:   true,
			KeepResources: true,
		}
		objectValue, err := plugin.UnmarshalProperties(resp.Object, marshalOpts)
		if err == nil {
			contract.IgnoreError(i.setVariable("pulumi", resource.NewProperty(objectValue)))
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

func unwrap(value resource.PropertyValue) resource.PropertyValue {
	if value.IsOutput() {
		return unwrap(value.OutputValue().Element)
	}
	if value.IsSecret() {
		return unwrap(value.SecretValue().Element)
	}
	return value
}

func unwrapResource(value resource.PropertyValue) (string, resource.PropertyValue, error) {
	value = unwrap(value)
	if !value.IsObject() {
		return "", resource.PropertyValue{}, fmt.Errorf("expected resource object, got %s", value.TypeString())
	}
	obj := value.ObjectValue()
	urnVal, ok := obj["urn"]
	if !ok || urnVal.IsNull() || urnVal.IsComputed() || !urnVal.IsString() {
		return "", resource.PropertyValue{}, fmt.Errorf("expected resource object with known urn property")
	}
	idVal, ok := obj["id"]
	if !ok || idVal.IsNull() {
		return "", resource.PropertyValue{}, fmt.Errorf("expected resource object with id property of type string")
	}

	if !idVal.IsComputed() && !idVal.IsString() {
		return "", resource.PropertyValue{}, fmt.Errorf("expected resource object with id property of type string")
	}

	return urnVal.StringValue(), idVal, nil
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
		KeepUnknowns:  true,
		KeepSecrets:   true,
		KeepResources: true,
	}
	obj, err := plugin.MarshalProperties(inputs, marshalOpts)
	if err != nil {
		return err
	}
	// The rest of this method can send output values
	marshalOpts.KeepOutputValues = true

	custom := true
	if res.Schema != nil {
		custom = !res.Schema.IsComponent
	}

	schemaResource, err := i.lookupResource(ctx, res.Token)
	if err != nil {
		return fmt.Errorf("lookup resource schema for token %s: %w", res.Token, err)
	}
	token := res.Token
	if schemaResource != nil {
		token = schemaResource.Token
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
		Type:                 token,
		Name:                 res.LogicalName(),
		Custom:               custom,
		Object:               obj,
		PropertyDependencies: propertyDependencies,
		Dependencies:         dependencies,
	}

	if res.Options != nil {
		if res.Options.AdditionalSecretOutputs != nil {
			additionalSecretOutputs, diags := i.evalExpression(res.Options.AdditionalSecretOutputs)
			if diags.HasErrors() {
				return diags
			}
			if !additionalSecretOutputs.IsNull() && !additionalSecretOutputs.IsComputed() {
				if !additionalSecretOutputs.IsArray() {
					return fmt.Errorf("additionalSecretOutputs must be an array of strings")
				}
				var additionalSecretOutputKeys []string
				for _, v := range additionalSecretOutputs.ArrayValue() {
					if v.IsNull() || v.IsComputed() {
						continue
					}
					if !v.IsString() {
						return fmt.Errorf("additionalSecretOutputs must be an array of strings")
					}
					additionalSecretOutputKeys = append(additionalSecretOutputKeys, v.StringValue())
				}
				request.AdditionalSecretOutputs = additionalSecretOutputKeys
			}
		}
		if res.Options.Aliases != nil {
			aliases, diags := i.evalExpression(res.Options.Aliases)
			if diags.HasErrors() {
				return diags
			}
			if !aliases.IsNull() && !aliases.IsComputed() {
				if !aliases.IsArray() {
					return fmt.Errorf("aliases must be an array of strings or alias objects")
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
							return err
						}
						err = setString("type", func(typ string) { aliasOpt.Type = typ })
						if err != nil {
							return err
						}

						noParent, ok := obj["noParent"]
						if ok && !noParent.IsNull() && !noParent.IsComputed() {
							if !noParent.IsBool() {
								return fmt.Errorf("noParent must be a boolean")
							}
							aliasOpt.Parent = &pulumirpc.Alias_Spec_NoParent{
								NoParent: noParent.BoolValue(),
							}
						}

						parent, ok := obj["parent"]
						if ok && !parent.IsNull() && !parent.IsComputed() {
							urn, _, err := unwrapResource(parent)
							if err != nil {
								return fmt.Errorf("parent: %w", err)
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
						return fmt.Errorf("aliases must be an array of strings or alias objects")
					}
				}
				request.Aliases = aliasOpts
			}
		}
		if res.Options.DependsOn != nil {
			dependsOn, diags := i.evalExpression(res.Options.DependsOn)
			if diags.HasErrors() {
				return diags
			}
			if !dependsOn.IsNull() && !dependsOn.IsComputed() {
				if !dependsOn.IsArray() {
					return fmt.Errorf("dependsOn must be an array of resource objects")
				}
				var dependsOnUrns []string
				for _, v := range dependsOn.ArrayValue() {
					if v.IsNull() || v.IsComputed() {
						continue
					}
					urn, _, err := unwrapResource(v)
					if err != nil {
						return fmt.Errorf("dependsOn: %w", err)
					}
					dependsOnUrns = append(dependsOnUrns, urn)
				}
				request.Dependencies = append(request.Dependencies, dependsOnUrns...)
			}
		}
		if res.Options.ImportID != nil {
			importID, diags := i.evalExpression(res.Options.ImportID)
			if diags.HasErrors() {
				return diags
			}
			if !importID.IsNull() && !importID.IsComputed() {
				if !importID.IsString() {
					return fmt.Errorf("import must be a string")
				}
				request.ImportId = importID.StringValue()
			}
		}
		if res.Options.IgnoreChanges != nil {
			ignoreChanges, diags := i.evalExpression(res.Options.IgnoreChanges)
			if diags.HasErrors() {
				return diags
			}
			if !ignoreChanges.IsNull() && !ignoreChanges.IsComputed() {
				if !ignoreChanges.IsArray() {
					return fmt.Errorf("ignoreChanges must be an array of strings")
				}
				icopt := []string{}
				for _, v := range ignoreChanges.ArrayValue() {
					if v.IsNull() || v.IsComputed() {
						continue
					}
					if !v.IsString() {
						return fmt.Errorf("ignoreChanges must be an array of strings")
					}
					icopt = append(icopt, v.StringValue())
				}
				request.IgnoreChanges = icopt
			}
		}
		if res.Options.Protect != nil {
			protect, diags := i.evalExpression(res.Options.Protect)
			if diags.HasErrors() {
				return diags
			}
			if !protect.IsComputed() {
				var popt *bool
				if protect.IsBool() {
					b := protect.BoolValue()
					popt = &b
				} else if !protect.IsNull() {
					return fmt.Errorf("protect must be a boolean or null")
				}
				request.Protect = popt
			}
		}
		if res.Options.ReplaceWith != nil {
			replaceWith, diags := i.evalExpression(res.Options.ReplaceWith)
			if diags.HasErrors() {
				return diags
			}
			if !replaceWith.IsNull() && !replaceWith.IsComputed() {
				if !replaceWith.IsArray() {
					return fmt.Errorf("replaceWith must be an array of resources")
				}
				var rwopt []string
				for _, v := range replaceWith.ArrayValue() {
					if v.IsNull() || v.IsComputed() {
						continue
					}
					urn, _, err := unwrapResource(v)
					if err != nil {
						return fmt.Errorf("replaceWith: %w", err)
					}
					rwopt = append(rwopt, urn)
				}
				request.ReplaceWith = rwopt
			}
		}
		if res.Options.ReplaceOnChanges != nil {
			replaceOnChanges, diags := i.evalExpression(res.Options.ReplaceOnChanges)
			if diags.HasErrors() {
				return diags
			}
			if !replaceOnChanges.IsNull() && !replaceOnChanges.IsComputed() {
				if !replaceOnChanges.IsArray() {
					return fmt.Errorf("replaceOnChanges must be an array of strings")
				}
				rocopt := []string{}
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
		if res.Options.RetainOnDelete != nil {
			retain, diags := i.evalExpression(res.Options.RetainOnDelete)
			if diags.HasErrors() {
				return diags
			}
			if !retain.IsNull() && !retain.IsComputed() {
				var retainOnDelete *bool
				if retain.IsBool() {
					b := retain.BoolValue()
					retainOnDelete = &b
				} else {
					return fmt.Errorf("retainOnDelete must be a boolean or null")
				}
				request.RetainOnDelete = retainOnDelete
			}
		}
		if res.Options.Version != nil {
			version, diags := i.evalExpression(res.Options.Version)
			if diags.HasErrors() {
				return diags
			}
			if !version.IsNull() && !version.IsComputed() {
				if !version.IsString() {
					return fmt.Errorf("version must be a string")
				}
				request.Version = version.StringValue()
			}
		}
		if res.Options.DeleteBeforeReplace != nil {
			dbr, diags := i.evalExpression(res.Options.DeleteBeforeReplace)
			if diags.HasErrors() {
				return diags
			}
			if !dbr.IsNull() && !dbr.IsComputed() {
				if dbr.IsBool() {
					request.DeleteBeforeReplace = dbr.BoolValue()
					request.DeleteBeforeReplaceDefined = true
				} else if !dbr.IsNull() {
					return fmt.Errorf("deleteBeforeReplace must be a boolean or null")
				}
			}
		}
		if res.Options.DeletedWith != nil {
			deletedWith, diags := i.evalExpression(res.Options.DeletedWith)
			if diags.HasErrors() {
				return diags
			}
			if !deletedWith.IsNull() && !deletedWith.IsComputed() {
				urn, _, err := unwrapResource(deletedWith)
				if err != nil {
					return fmt.Errorf("deletedWith: %w", err)
				}
				request.DeletedWith = string(urn)
			}
		}
		if res.Options.PluginDownloadURL != nil {
			downloadURL, diags := i.evalExpression(res.Options.PluginDownloadURL)
			if diags.HasErrors() {
				return diags
			}
			if !downloadURL.IsNull() && !downloadURL.IsComputed() {
				if !downloadURL.IsString() {
					return fmt.Errorf("pluginDownloadURL must be a string")
				}
				request.PluginDownloadURL = downloadURL.StringValue()
			}
		}
		if res.Options.Parent != nil {
			parent, diags := i.evalExpression(res.Options.Parent)
			if diags.HasErrors() {
				return diags
			}
			if !parent.IsNull() && !parent.IsComputed() {
				urn, _, err := unwrapResource(parent)
				if err != nil {
					return fmt.Errorf("parent: %w", err)
				}
				request.Parent = string(urn)
			}
		}
		if res.Options.Provider != nil {
			provider, diags := i.evalExpression(res.Options.Provider)
			if diags.HasErrors() {
				return diags
			}
			if !provider.IsNull() && !provider.IsComputed() {
				urn, id, err := unwrapResource(provider)
				if err != nil {
					return fmt.Errorf("provider: %w", err)
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
			providers, diags := i.evalExpression(res.Options.Providers)
			if diags.HasErrors() {
				return diags
			}
			if !providers.IsNull() && !providers.IsComputed() {
				// Providers is either a list of provider objects or a map of provider name to provider objects. We need to support both forms and translate the list into the map form expected by the RPC.
				psopt := map[string]string{}
				if providers.IsObject() {
					for k, v := range providers.ObjectValue() {
						urn, id, err := unwrapResource(v)
						if err != nil {
							return fmt.Errorf("providers: %w", err)
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
							return fmt.Errorf("providers: %w", err)
						}
						typ := resource.URN(urn).Type()
						components := strings.Split(string(typ), ":")
						pkg := components[2]

						var idstr string
						if id.IsString() {
							idstr = id.StringValue()
						} else {
							idstr = plugin.UnknownStringValue
						}
						psopt[string(pkg)] = fmt.Sprintf("%s::%s", urn, idstr)
					}
				} else {
					return fmt.Errorf("providers must be an array of provider objects or a map of provider name to provider objects")
				}
				request.Providers = psopt
			}
		}
		if res.Options.HideDiffs != nil {
			hideDiffs, diags := i.evalExpression(res.Options.HideDiffs)
			if diags.HasErrors() {
				return diags
			}
			if !hideDiffs.IsNull() && !hideDiffs.IsComputed() {
				if !hideDiffs.IsArray() {
					return fmt.Errorf("hideDiffs must be an array of strings")
				}
				hdopt := []string{}
				for _, v := range hideDiffs.ArrayValue() {
					if v.IsNull() || v.IsComputed() {
						continue
					}
					if !v.IsString() {
						return fmt.Errorf("hideDiffs must be an array of strings")
					}
					hdopt = append(hdopt, v.StringValue())
				}
				request.HideDiffs = hdopt
			}
		}
	}

	// Default parent to the stack if not specified
	if request.Parent == "" {
		request.Parent = i.stackURN
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
	outputs["__name"] = resource.NewProperty(request.Name)
	outputs["__type"] = resource.NewProperty(request.Type)

	// We need to ensure _all_ resource outputs exist in the output object so any the provider didn't send back we default to unknown here.
	if schemaResource != nil {
		for _, prop := range schemaResource.Properties {
			key := resource.PropertyKey(prop.Name)
			if _, ok := outputs[key]; !ok {
				outputs[key] = resource.NewProperty(resource.Computed{Element: resource.NewProperty("")})
			}
		}
	}

	result := resource.NewOutputProperty(resource.Output{
		Element:      resource.NewProperty(outputs),
		Dependencies: []resource.URN{resource.URN(resp.GetUrn())},
		Known:        true,
	})

	if err := i.setVariable(res.Name(), result); err != nil {
		return err
	}
	return nil
}

func (i *Interpreter) registerStackOutputs(ctx context.Context, outputs resource.PropertyMap) error {
	if i.stackURN == "" {
		return errors.New("missing stack URN")
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

func (i *Interpreter) setVariable(name string, value resource.PropertyValue) error {
	ctyValue, err := propertyValueToCty(value)
	if err != nil {
		return err
	}
	i.evalLock.Lock()
	i.evalContext.Variables[name] = ctyValue
	i.evalLock.Unlock()
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
