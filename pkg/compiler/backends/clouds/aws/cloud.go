// Copyright 2016 Marapongo, Inc. All rights reserved.

package aws

import (
	"fmt"

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

func (c *awsCloud) Diag() diag.Sink {
	return c.d
}

func (c *awsCloud) CodeGen(comp core.Compiland) {
	// For now, this routine simply generates the equivalent CloudFormation stack for the input.  Eventually this needs
	// to do a whole lot more, which the following running list of TODOs will serve as a reminder about:
	// TODO: perform delta analysis so that we can emit changesets:
	//     http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/using-cfn-updating-stacks-changesets.html
	// TODO: allow for a "dry-run" mode that queries the target, checks things like limits, shows what will be done.
	// TODO: prepare full deployment packages (e.g., tarballs of code, Docker images, etc).
	nm := c.genStackName(comp)
	cf := c.genStackTemplate(comp)
	if c.Diag().Errors() == 0 {
		// TODO: actually save this (and any other outputs) to disk, rather than spewing to STDOUT.
		y, err := yaml.Marshal(cf)
		if err != nil {
			c.Diag().Errorf(ErrorMarshalingCloudFormationTemplate.WithDocument(comp.Doc), err)
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
		makeAWSFriendlyName(comp.Target.Name, true), makeAWSFriendlyName(string(comp.Stack.Name), true))
	util.Assert(IsValidStackName(nm))
	return nm
}

// genServiceName creates a name for the service, which must be unique within a single CloudFormation template.
func (c *awsCloud) genServiceName(stack *ast.Stack, svc *ast.Service) cfLogicalID {
	nm := fmt.Sprintf("%v%v",
		makeAWSFriendlyName(string(stack.Name), true), makeAWSFriendlyName(string(svc.Name), true))
	util.Assert(IsValidLogicalID(nm))
	return cfLogicalID(nm)
}

// genStackTemplate creates a CloudFormation template for an entire stack and all of its services.
func (c *awsCloud) genStackTemplate(comp core.Compiland) *cfTemplate {
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
	for _, name := range ast.StableServices(comp.Stack.Services.Private) {
		svc := comp.Stack.Services.Private[name]
		if res := c.genServiceTemplate(comp, &svc); res != nil {
			nm := c.genServiceName(comp.Stack, &svc)
			cf.Resources[nm] = *res
		}
	}
	for _, name := range ast.StableServices(comp.Stack.Services.Public) {
		svc := comp.Stack.Services.Public[name]
		if res := c.genServiceTemplate(comp, &svc); res != nil {
			nm := c.genServiceName(comp.Stack, &svc)
			cf.Resources[nm] = *res
		}
	}

	// TODO: emit output exports (public services) that can be consumed by other stacks.

	return cf
}

// genServiceTemplate creates a CloudFormation resource for a single service.
func (c *awsCloud) genServiceTemplate(comp core.Compiland, svc *ast.Service) *cfResource {
	// Code-generation differs greatly for the various service types.  There are three categories:
	//		1) A Mu primitive: these have very specific manifestations to accomplish the desired Mu semantics.
	//		2) An AWS-specific extension type: these largely just pass-through CloudFormation goo that we will emit.
	//		3) A reference to another Stack: these just instantiate those Stacks and reference their outputs.

	switch svc.BoundType {
	case predef.MuContainer:
		return c.genMuContainerServiceTemplate(comp, svc)
	case predef.MuGateway:
		return c.genMuGatewayServiceTemplate(comp, svc)
	case predef.MuFunc:
		return c.genMuFuncServiceTemplate(comp, svc)
	case predef.MuEvent:
		return c.genMuEventServiceTemplate(comp, svc)
	case predef.MuVolume:
		return c.genMuVolumeServiceTemplate(comp, svc)
	case predef.MuAutoscaler:
		return c.genMuAutoscalerServiceTemplate(comp, svc)
	case predef.MuExtension:
		return c.genMuExtensionServiceTemplate(comp, predef.AsMuExtensionService(svc))
	default:
		return c.genStackServiceTemplate(comp, svc)
	}
}

func (c *awsCloud) genMuContainerServiceTemplate(comp core.Compiland, svc *ast.Service) *cfResource {
	glog.Fatalf("%v service types are not yet supported\n", svc.Name)
	return nil
}

func (c *awsCloud) genMuGatewayServiceTemplate(comp core.Compiland, svc *ast.Service) *cfResource {
	glog.Fatalf("%v service types are not yet supported\n", svc.Name)
	return nil
}

func (c *awsCloud) genMuFuncServiceTemplate(comp core.Compiland, svc *ast.Service) *cfResource {
	glog.Fatalf("%v service types are not yet supported\n", svc.Name)
	return nil
}

func (c *awsCloud) genMuEventServiceTemplate(comp core.Compiland, svc *ast.Service) *cfResource {
	glog.Fatalf("%v service types are not yet supported\n", svc.Name)
	return nil
}

func (c *awsCloud) genMuVolumeServiceTemplate(comp core.Compiland, svc *ast.Service) *cfResource {
	glog.Fatalf("%v service types are not yet supported\n", svc.Name)
	return nil
}

func (c *awsCloud) genMuAutoscalerServiceTemplate(comp core.Compiland, svc *ast.Service) *cfResource {
	glog.Fatalf("%v service types are not yet supported\n", svc.Name)
	return nil
}

// CloudFormationExtensionProvider, when used with mu/extension, allows a stack to directly generate arbitrary
// CloudFormation templating as the output.  This happens after Mu templates have been expanded, allowing stack
// properties, target environments, and so on, to be leveraged in the way these templates are generated.
const CloudFormationExtensionProvider = "aws/cf"
const CloudFormationExtensionProviderResource = "resource"
const CloudFormationExtensionProviderTypeField = "Type"
const CloudFormationExtensionProviderPropertiesField = "Properties"

func (c *awsCloud) genMuExtensionServiceTemplate(comp core.Compiland, svc *predef.MuExtensionService) *cfResource {
	switch svc.Provider {
	case CloudFormationExtensionProvider:
		// The AWS CF extension provider simply creates a CF resource out of the provided template.

		var res map[string]interface{}
		r, ok := svc.Extra[CloudFormationExtensionProviderResource]
		if ok {
			res, ok = r.(map[string]interface{})
			if !ok {
				c.Diag().Errorf(errors.ErrorIncorrectExtensionPropertyType.WithDocument(comp.Doc),
					CloudFormationExtensionProviderResource, "string-keyed map")
				return nil
			}
		} else {
			c.Diag().Errorf(errors.ErrorMissingExtensionProperty.WithDocument(comp.Doc),
				CloudFormationExtensionProviderTypeField)
			return nil
		}

		var resType string
		t, ok := res[CloudFormationExtensionProviderTypeField]
		if ok {
			resType, ok = t.(string)
			if !ok {
				c.Diag().Errorf(errors.ErrorIncorrectExtensionPropertyType.WithDocument(comp.Doc),
					fmt.Sprintf("%v.%v", CloudFormationExtensionProviderResource,
						CloudFormationExtensionProviderTypeField), "string")
				return nil
			}
		} else {
			c.Diag().Errorf(errors.ErrorMissingExtensionProperty.WithDocument(comp.Doc),
				fmt.Sprintf("%v.%v", CloudFormationExtensionProviderResource,
					CloudFormationExtensionProviderTypeField))
			return nil
		}

		var resProps map[string]interface{}
		p, ok := res[CloudFormationExtensionProviderPropertiesField]
		if ok {
			resProps, ok = p.(map[string]interface{})
			if !ok {
				c.Diag().Errorf(errors.ErrorIncorrectExtensionPropertyType.WithDocument(comp.Doc),
					fmt.Sprintf("%v.%v", CloudFormationExtensionProviderResource,
						CloudFormationExtensionProviderPropertiesField), "string-keyed map")
				return nil
			}
		} else {
			c.Diag().Errorf(errors.ErrorMissingExtensionProperty.WithDocument(comp.Doc),
				fmt.Sprintf("%v.%v", CloudFormationExtensionProviderResource,
					CloudFormationExtensionProviderPropertiesField))
			return nil
		}

		return &cfResource{cfResourceType(resType), cfResourceProperties(resProps)}
	default:
		c.Diag().Errorf(errors.ErrorUnrecognizedExtensionProvider.WithDocument(comp.Doc), svc.Provider)
	}

	return nil
}

// genStackServiceTemplate generates code for a general-purpose Stack service reference.
func (c *awsCloud) genStackServiceTemplate(comp core.Compiland, svc *ast.Service) *cfResource {
	glog.Fatalf("%v service types are not yet supported\n", svc.Name)
	return nil
}
