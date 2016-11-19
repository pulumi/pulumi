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
	"github.com/marapongo/mu/pkg/errors"
)

// New returns a fresh instance of an AWS Cloud implementation.  This targets "native AWS" for the code-gen outputs.
// This primarily means CloudFormation as the stack templating output, and idiomatic AWS services like S3, DynamoDB,
// Lambda, and so on, for the actual services in those stack templates.
//
// For more details, see https://github.com/marapongo/mu/blob/master/docs/targets.md#amazon-web-services-aws
func New() clouds.Cloud {
	return &awsCloud{}
}

type awsCloud struct {
	clouds.Cloud
	// TODO: support cloud provider options (e.g., ranging from simple like YAML vs. JSON to complex like IAM).
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
	// TODO: actually save this (and any other outputs) to disk, rather than spewing to STDOUT.
	y, err := yaml.Marshal(cf)
	if err != nil {
		comp.Diag.Errorf(ErrorMarshalingCloudFormationTemplate.WithDocument(comp.Doc), err)
		return
	}
	fmt.Printf("%v:\n", nm)
	fmt.Println(string(y))
}

// genClusterTemplate creates a CloudFormation template for a standard overall cluster.
func (c *awsCloud) genClusterTemplate(comp core.Compiland) *cfTemplate {
	// TODO: this.
	return nil
}

// genStackName creates a name for the stack, which must be globally unique within an account.
func (c *awsCloud) genStackName(comp core.Compiland) string {
	return fmt.Sprintf("MuStack-%v-%v", comp.Target.Name, comp.Stack.Name)
}

// genStackTemplate creates a CloudFormation template for an entire stack and all of its services.
func (c *awsCloud) genStackTemplate(comp core.Compiland) *cfTemplate {
	// Allocate a new template object that we will populate and return.
	cf := &cfTemplate{
		AWSTemplateFormatVersion: cfVersion,
		Description:              comp.Stack.Description,
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
		cf.Resources[string(svc.Name)] = *c.genServiceTemplate(comp, &svc)
	}
	for _, name := range ast.StableServices(comp.Stack.Services.Public) {
		svc := comp.Stack.Services.Public[name]
		cf.Resources[string(svc.Name)] = *c.genServiceTemplate(comp, &svc)
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
const CloudFormationExtensionProviderTypeField = "type"
const CloudFormationExtensionProviderPropertiesField = "type"

func (c *awsCloud) genMuExtensionServiceTemplate(comp core.Compiland, svc *predef.MuExtensionService) *cfResource {
	switch svc.Provider {
	case CloudFormationExtensionProvider:
		// The AWS CF extension provider simply creates a CF resource out of the provided template.

		var resType cfResourceType
		t, ok := svc.Extra[CloudFormationExtensionProviderTypeField]
		if ok {
			resType, ok = t.(cfResourceType)
			if !ok {
				c.Diag().Errorf(errors.ErrorIncorrectExtensionPropertyType.WithDocument(comp.Doc),
					CloudFormationExtensionProviderTypeField, "string")
			}
		} else {
			c.Diag().Errorf(errors.ErrorMissingExtensionProperty.WithDocument(comp.Doc),
				CloudFormationExtensionProviderTypeField)
		}

		var resProps cfResourceProperties
		p, ok := svc.Extra[CloudFormationExtensionProviderPropertiesField]
		if ok {
			resProps, ok = p.(cfResourceProperties)
			if !ok {
				c.Diag().Errorf(errors.ErrorIncorrectExtensionPropertyType.WithDocument(comp.Doc),
					CloudFormationExtensionProviderPropertiesField, "string-keyed map")
			}
		} else {
			c.Diag().Errorf(errors.ErrorMissingExtensionProperty.WithDocument(comp.Doc),
				CloudFormationExtensionProviderPropertiesField)
		}

		return &cfResource{resType, resProps}
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
