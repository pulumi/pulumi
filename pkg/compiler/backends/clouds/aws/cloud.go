// Copyright 2016 Marapongo, Inc. All rights reserved.

package aws

import (
	"fmt"
	"reflect"

	"github.com/ghodss/yaml"
	"github.com/golang/glog"

	"github.com/marapongo/mu/pkg/ast"
	"github.com/marapongo/mu/pkg/compiler/backends/clouds"
	"github.com/marapongo/mu/pkg/compiler/core"
	"github.com/marapongo/mu/pkg/compiler/predef"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/errors"
	"github.com/marapongo/mu/pkg/util"
)

// New returns a fresh instance of an AWS Cloud implementation.  This targets "native AWS" for the code-gen outputs.
// This primarily means CloudFormation as the stack templating output, and idiomatic AWS services like S3, DynamoDB,
// Lambda, and so on, for the actual services in those stack templates.
//
// For more details, see https://github.com/marapongo/mu/blob/master/docs/targets.md#amazon-web-services-aws
func New(d diag.Sink) clouds.Cloud {
	return &awsCloud{d: d}
}

type awsCloud struct {
	clouds.Cloud
	d diag.Sink
	// TODO: support cloud provider options (e.g., ranging from simple like YAML vs. JSON to complex like IAM).
}

func (c *awsCloud) Arch() clouds.Arch {
	return clouds.AWS
}

func (c *awsCloud) Diag() diag.Sink {
	return c.d
}

func (c *awsCloud) CodeGen(comp core.Compiland) {
	glog.Infof("%v CodeGen: cluster=%v stack=%v", clouds.Names[c.Arch()], comp.Cluster.Name, comp.Stack.Name)
	if glog.V(2) {
		defer glog.Infof("%v CodeGen: cluster=%v stack=%v completed w/ %v warnings and %v errors",
			clouds.Names[c.Arch()], comp.Cluster.Name, comp.Stack.Name,
			c.Diag().Warnings(), c.Diag().Errors())
	}

	// For now, this routine simply generates the equivalent CloudFormation stack for the input.  Eventually this needs
	// to do a whole lot more, which the following running list of TODOs will serve as a reminder about:
	// TODO: perform delta analysis so that we can emit changesets:
	//     http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/using-cfn-updating-stacks-changesets.html
	// TODO: allow for a "dry-run" mode that queries the target, checks things like limits, shows what will be done.
	// TODO: prepare full deployment packages (e.g., tarballs of code, Docker images, etc).
	nm := c.genStackName(comp)
	cf := c.genTemplate(comp)
	if c.Diag().Success() {
		// TODO: actually save this (and any other outputs) to disk, rather than spewing to STDOUT.
		y, err := yaml.Marshal(cf)
		if err != nil {
			c.Diag().Errorf(ErrorMarshalingCloudFormationTemplate.At(comp.Stack), err)
			return
		}
		fmt.Printf("# %v:\n", nm)
		fmt.Println(string(y))
	}
}

// genClusterTemplate creates a CloudFormation template for a standard overall cluster.
func (c *awsCloud) genClusterTemplate(comp core.Compiland) *cfTemplate {
	// TODO: this.
	return nil
}

// genStackName creates a name for the stack, which must be globally unique within an account.
func (c *awsCloud) genStackName(comp core.Compiland) string {
	nm := fmt.Sprintf("MuStack-%v-%v",
		makeAWSFriendlyName(comp.Cluster.Name, true), makeAWSFriendlyName(string(comp.Stack.Name), true))
	util.Assert(IsValidCFStackName(nm))
	return nm
}

// genResourceID creates an ID for a resource, which must be unique within a single CloudFormation template.
func (c *awsCloud) genResourceID(stack *ast.Stack, svc *ast.Service) cfLogicalID {
	nm := fmt.Sprintf("%v%v",
		makeAWSFriendlyName(string(stack.Name), true), makeAWSFriendlyName(string(svc.Name), true))
	util.Assert(IsValidCFLogicalID(nm))
	return cfLogicalID(nm)
}

// genTemplate creates a CloudFormation template for an entire compiland and all of its services.
func (c *awsCloud) genTemplate(comp core.Compiland) *cfTemplate {
	// Allocate a new template object that we will populate and return.
	cf := &cfTemplate{
		AWSTemplateFormatVersion: cfVersion,
		Description:              comp.Stack.Description,
		Resources:                make(cfResources),
	}

	// TODO: add parameters.
	// TODO: due to the way we expand Mu templates, we don't leverage AWS CloudFormation parameters.  That's generally
	//     simpler, however, sometimes a customer may want the parameterization to persist (e.g., so they end up with
	//     a single CloudFormation template across multiple environments, say).  This extends to other templatization
	//     that would normally use CloudFormation's own conditionals.  It's possible we can just have a
	//     --skip-template-expansion mode that keeps the Mu templates and/or transforms them into AWS ones.

	// Emit the services.  Although services can depend on one another, the order in which we emit them here doesn't
	// matter.  The reason is that those dependencies are "runtime"-based and will get resolved elsewhere.
	privates, publics := c.genStackServiceTemplates(comp, comp.Stack)
	for nm, private := range privates {
		cf.Resources[nm] = private
	}
	for nm, public := range publics {
		cf.Resources[nm] = public
	}

	// TODO: emit output exports (public services) that can be consumed by other stacks.

	// TODO: we will need to consider whether to emit DependsOn attributes for capability references.  In many cases,
	//     we will consume them (emitting a Ref:: in the CF output), so there is no need.  In other cases, however, a
	//     service may depend on them "ambiently" -- e.g., by DNS name or worse, nothing visible to the system at all --
	//     in which case we will need to emit the DependsOn.  It'd be really nice if developers didn't do this by hand.

	return cf
}

// genStackServiceTemplates returns two maps of service templates, one for private, the other for public, services.
func (c *awsCloud) genStackServiceTemplates(comp core.Compiland, stack *ast.Stack) (cfResources, cfResources) {
	util.Assert(stack != nil)
	glog.V(4).Infof("Generating stack service templates: stack=%v private=%v public=%v",
		stack.Name, len(stack.Services.Private), len(stack.Services.Public))

	privates := make(cfResources)
	publics := make(cfResources)
	defer glog.V(5).Infof("Generated stack service templates: stack=%v private=%v public=%v",
		stack.Name, len(privates), len(publics))

	for _, name := range ast.StableServices(stack.Services.Private) {
		svc := stack.Services.Private[name]
		for nm, r := range c.genServiceTemplate(comp, stack, &svc) {
			privates[nm] = r
		}
	}

	for _, name := range ast.StableServices(stack.Services.Public) {
		svc := stack.Services.Public[name]
		for nm, r := range c.genServiceTemplate(comp, stack, &svc) {
			publics[nm] = r
		}
	}

	return privates, publics
}

// genServiceTemplate creates a CloudFormation resource for a single service.
func (c *awsCloud) genServiceTemplate(comp core.Compiland, stack *ast.Stack, svc *ast.Service) cfResources {
	util.Assert(svc.BoundType != nil)
	glog.V(4).Infof("Generating service templates: svc=%v type=%v", svc.Name, svc.BoundType.Name)

	// Code-generation differs greatly for the various service types.  There are three categories:
	//		1) A Mu primitive: these have very specific manifestations to accomplish the desired Mu semantics.
	//		2) An AWS-specific extension type: these largely just pass-through CloudFormation goo that we will emit.
	//		3) A reference to another Stack: these just instantiate those Stacks and reference their outputs.
	switch svc.BoundType {
	case predef.MuContainer:
		return c.genMuContainerServiceTemplate(comp, stack, svc)
	case predef.MuGateway:
		return c.genMuGatewayServiceTemplate(comp, stack, svc)
	case predef.MuFunc:
		return c.genMuFuncServiceTemplate(comp, stack, svc)
	case predef.MuEvent:
		return c.genMuEventServiceTemplate(comp, stack, svc)
	case predef.MuVolume:
		return c.genMuVolumeServiceTemplate(comp, stack, svc)
	case predef.MuAutoscaler:
		return c.genMuAutoscalerServiceTemplate(comp, stack, svc)
	case predef.MuExtension:
		return c.genMuExtensionServiceTemplate(comp, stack, predef.AsMuExtensionService(svc))
	default:
		return c.genOtherServiceTemplate(comp, stack, svc)
	}
}

func (c *awsCloud) genMuContainerServiceTemplate(comp core.Compiland, stack *ast.Stack, svc *ast.Service) cfResources {
	util.FailMF("%v service types are not yet supported (svc:  %v)\n", svc.BoundType.Name, svc.Name)
	return nil
}

func (c *awsCloud) genMuGatewayServiceTemplate(comp core.Compiland, stack *ast.Stack, svc *ast.Service) cfResources {
	util.FailMF("%v service types are not yet supported (svc:  %v)\n", svc.BoundType.Name, svc.Name)
	return nil
}

func (c *awsCloud) genMuFuncServiceTemplate(comp core.Compiland, stack *ast.Stack, svc *ast.Service) cfResources {
	util.FailMF("%v service types are not yet supported (svc:  %v)\n", svc.BoundType.Name, svc.Name)
	return nil
}

func (c *awsCloud) genMuEventServiceTemplate(comp core.Compiland, stack *ast.Stack, svc *ast.Service) cfResources {
	util.FailMF("%v service types are not yet supported (svc:  %v)\n", svc.BoundType.Name, svc.Name)
	return nil
}

func (c *awsCloud) genMuVolumeServiceTemplate(comp core.Compiland, stack *ast.Stack, svc *ast.Service) cfResources {
	util.FailMF("%v service types are not yet supported (svc:  %v)\n", svc.BoundType.Name, svc.Name)
	return nil
}

func (c *awsCloud) genMuAutoscalerServiceTemplate(comp core.Compiland, stack *ast.Stack, svc *ast.Service) cfResources {
	util.FailMF("%v service types are not yet supported (svc:  %v)\n", svc.BoundType.Name, svc.Name)
	return nil
}

// CloudFormationExtensionProvider, when used with mu/extension, allows a stack to directly generate arbitrary
// CloudFormation templating as the output.  This happens after Mu templates have been expanded, allowing stack
// properties, target environments, and so on, to be leveraged in the way these templates are generated.
const CloudFormationExtensionProvider = "aws/cf"

// CloudFormationExtensionProviderResource is the property that contains the AWS CF resource name (required).
const CloudFormationExtensionProviderResource = "resource"

// CloudFormationExtensionProviderDependsOn optionally lists other AWS CF resource IDs that this resource depends on.
const CloudFormationExtensionProviderDependsOn = "dependsOn"

// CloudFormationExtensionproviderProperties optionally contains the set of properties to auto-map (default is all).
const CloudFormationExtensionProviderProperties = "properties"

// CloudFormationExtensionproviderSkipProperties optionally contains a set of properties to skip in auto-mapping.
const CloudFormationExtensionProviderSkipProperties = "skipProperties"

// CloudFormationExtensionproviderExtraProperties contains an optional set of arbitrary properties to merge.
const CloudFormationExtensionProviderExtraProperties = "extraProperties"

func (c *awsCloud) genMuExtensionServiceTemplate(comp core.Compiland, stack *ast.Stack,
	svc *predef.MuExtensionService) cfResources {
	switch svc.Provider {
	case CloudFormationExtensionProvider:
		// The AWS CF extension provider simply creates a CF resource out of the provided template.  First, we extract
		// the resource type, which is simply the AWS CF resource type name to emit directly, unmanipulated.
		var resType string
		r, ok := svc.Props[CloudFormationExtensionProviderResource]
		if ok {
			resType, ok = r.(string)
			if !ok {
				c.Diag().Errorf(errors.ErrorIncorrectExtensionPropertyType.At(stack),
					CloudFormationExtensionProviderResource, reflect.TypeOf(r), "string")
				return nil
			}
		} else {
			c.Diag().Errorf(errors.ErrorMissingExtensionProperty.At(stack),
				CloudFormationExtensionProviderResource)
			return nil
		}

		// See if there are a set of properties to auto-map; if missing, the default is "all of them."
		var auto map[string]bool
		if au, ok := svc.Props[CloudFormationExtensionProviderProperties]; ok {
			if aups, ok := au.([]string); ok {
				auto = make(map[string]bool)
				for _, p := range aups {
					auto[p] = true
				}
			} else {
				c.Diag().Errorf(errors.ErrorIncorrectExtensionPropertyType.At(stack),
					CloudFormationExtensionProviderProperties, reflect.TypeOf(au), "[]string")
			}
		}

		// See if there are any properties to skip during auto-mapping.
		var skip map[string]bool
		if sk, ok := svc.Props[CloudFormationExtensionProviderSkipProperties]; ok {
			skip = make(map[string]bool)
			if ska, ok := sk.([]string); ok {
				for _, s := range ska {
					skip[s] = true
				}
			} else {
				c.Diag().Errorf(errors.ErrorIncorrectExtensionPropertyType.At(stack),
					CloudFormationExtensionProviderSkipProperties, reflect.TypeOf(sk), "[]string")
			}
		}

		// Next, we perform a straighforward auto-mapping from Mu stack properties to the equivalent CF properties.
		resProps := make(cfResourceProperties)
		for _, name := range ast.StableProperties(stack.Properties) {
			if (auto == nil || auto[name]) && (skip == nil || !skip[name]) {
				if p, has := svc.Service.Props[name]; has {
					pname := makeAWSFriendlyName(name, true)
					resProps[pname] = p
				}
			}
		}

		// Next, if there are any "extra" properties, merge them in with the existing map.
		if ex, ok := svc.Props[CloudFormationExtensionProviderExtraProperties]; ok {
			if extra, ok := ex.(map[string]interface{}); ok {
				for _, exname := range ast.StableKeys(extra) {
					v := extra[exname]
					// If there is an existing property, we can (possibly) merge it, for maps and slices (using some
					// reflection voodoo).  For all other types, issue a warning.
					if exist, has := resProps[exname]; has {
						merged := true
						switch reflect.TypeOf(exist).Kind() {
						case reflect.Map:
							// Merge two maps, provided both are maps; if any conflicting keys exist, bail out.
							if reflect.TypeOf(v).Kind() == reflect.Map {
								vm := reflect.ValueOf(v).Interface().(map[string]interface{})
								em := reflect.ValueOf(exist).Interface().(map[string]interface{})
								for k, v := range vm {
									if _, has := em[k]; has {
										merged = false
									} else {
										em[k] = v
									}
								}
							} else {
								merged = false
							}
						case reflect.Slice:
							// Merge two slices, provided both are slices.
							if reflect.TypeOf(v).Kind() == reflect.Slice {
								reflect.AppendSlice(reflect.ValueOf(exist), reflect.ValueOf(v))
							} else {
								merged = false
							}
						default:
							merged = false
						}
						if !merged {
							c.Diag().Errorf(ErrorDuplicateExtraProperty.At(stack), exname)
						}
					} else {
						resProps[exname] = v
					}
				}
			} else {
				c.Diag().Errorf(errors.ErrorIncorrectExtensionPropertyType.At(stack),
					CloudFormationExtensionProviderExtraProperties, reflect.TypeOf(ex), "map[string]interface{}")
			}
		}

		// If there are any explicit dependencies listed, we need to fish them out and add them.
		var resDeps []cfLogicalID
		if do, ok := svc.Props[CloudFormationExtensionProviderDependsOn]; ok {
			resDeps = make([]cfLogicalID, 0)
			if doa, ok := do.([]string); ok {
				for _, d := range doa {
					resDeps = append(resDeps, cfLogicalID(d))
				}
			} else {
				c.Diag().Errorf(errors.ErrorIncorrectExtensionPropertyType.At(stack),
					CloudFormationExtensionProviderDependsOn, reflect.TypeOf(do), "[]string")
			}
		}

		// Finally, generate an ID from the service's name, and return the result.
		id := c.genResourceID(stack, &svc.Service)
		return cfResources{
			id: cfResource{
				Type:       cfResourceType(resType),
				Properties: cfResourceProperties(resProps),
				DependsOn:  resDeps,
			},
		}
	default:
		c.Diag().Errorf(errors.ErrorUnrecognizedExtensionProvider.At(stack), svc.Provider)
	}

	return nil
}

// genOtherServiceTemplate generates code for a general-purpose Stack service reference.
func (c *awsCloud) genOtherServiceTemplate(comp core.Compiland, stack *ast.Stack, svc *ast.Service) cfResources {
	// Instantiate and textually include the BoundStack into our current template.
	// TODO: consider an option where a Stack can become a distinct CloudFormation Stack, and then reference it by
	//     name.  This would be a terrible default, because we'd end up with dozens of CloudFormation Stacks for even
	//     the simplest of Mu Stacks.  Especially because many Mu Stacks are single-Service.  Perhaps we could come
	//     up with some clever default, like multi-Service Mu Stacks map to CloudFormation Stacks, and single-Service
	//     ones don't, however I'm not yet convinced this is the right path.  So, for now, we keep it simple.
	util.Assert(svc.BoundType != nil)
	glog.V(4).Infof("Generating \"other\" service template: svc=%v type=%v", svc.Name, svc.BoundType.Name)

	all := make(cfResources)
	privates, publics := c.genStackServiceTemplates(comp, svc.BoundType)
	defer glog.V(5).Infof("Generated \"other\" service template: svc=%v type=%v private=%v public=%v all=%v",
		svc.Name, svc.BoundType.Name, len(privates), len(publics), len(all))

	// Copy all of the returned resources to a single map and return it.
	for nm, private := range privates {
		all[nm] = private
	}
	for nm, public := range publics {
		all[nm] = public
	}
	return all
}
