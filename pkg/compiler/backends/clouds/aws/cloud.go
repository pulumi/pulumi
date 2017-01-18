// Copyright 2016 Marapongo, Inc. All rights reserved.

package aws

import (
	"fmt"
	"reflect"

	"github.com/golang/glog"

	"github.com/marapongo/mu/pkg/ast"
	"github.com/marapongo/mu/pkg/ast/conv"
	"github.com/marapongo/mu/pkg/compiler/backends/clouds"
	"github.com/marapongo/mu/pkg/compiler/core"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/encoding"
	"github.com/marapongo/mu/pkg/errors"
	"github.com/marapongo/mu/pkg/util/contract"
)

// New returns a fresh instance of an AWS Cloud implementation.  This targets "native AWS" for the code-gen outputs.
// This primarily means CloudFormation as the stack templating output, and idiomatic AWS services like S3, DynamoDB,
// Lambda, and so on, for the actual services in those stack templates.
//
// For more details, see https://github.com/marapongo/mu/blob/master/docs/targets.md#amazon-web-services-aws
func New(d diag.Sink, opts Options) clouds.Cloud {
	// If no format was specified, pick YAML.
	var m encoding.Marshaler
	if opts.Ext == "" {
		m = encoding.YAML
	} else {
		m = encoding.Marshalers[opts.Ext]
	}

	return &awsCloud{d: d, m: m, opts: opts}
}

// Options controls the behavior of the AWS Cloud backend implementation.
type Options struct {
	Ext string // the output format to generate (e.g., ".yaml", ".json")
}

type awsCloud struct {
	clouds.Cloud
	d    diag.Sink
	m    encoding.Marshaler
	opts Options
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
		y, err := c.m.Marshal(cf)
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
	contract.Assert(IsValidCFStackName(nm))
	return nm
}

// genResourceID creates an ID for a resource, which must be unique within a single CloudFormation template.
func (c *awsCloud) genResourceID(stack *ast.Stack, svc *ast.Service) cfLogicalID {
	nm := fmt.Sprintf("%v%v",
		makeAWSFriendlyName(string(svc.Name), true), makeAWSFriendlyName(string(stack.Name), true))
	contract.Assert(IsValidCFLogicalID(nm))
	return cfLogicalID(nm)
}

// genResourceDependsID discovers the CF logical ID used to reference the selected service in the same stack.
func (c *awsCloud) genResourceDependsID(ref *ast.ServiceRef) cfLogicalID {
	// First, we need to dig deep down to figure out what actual AWS resource this dependency is on.
	// TODO: support cross-stack references.
	sel := ref.Selected
	for {
		if sel.BoundType.Name == cfIntrinsicName {
			break
		}

		// TODO: this works "one-level deep"; however, we will need to figure out a scheme for logical dependencies;
		//     that is, dependencies on stacks that are merely a composition of many other stacks.
		contract.Assertf(len(sel.BoundType.Services.Public) == 1,
			"expected service type '%v' to export a single public service; got %v",
			sel.BoundType.Name, len(sel.BoundType.Services.Public))
		for _, s := range sel.BoundType.Services.Public {
			sel = s
			break
		}
	}
	return c.genResourceID(ref.Service.BoundType, sel)
}

// genResourceDependsRef creates a reference to another resource inside of this same stack.  For more information, see
// http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/intrinsic-function-reference-ref.html.
func (c *awsCloud) genResourceDependsRef(ref *ast.ServiceRef) interface{} {
	id := c.genResourceDependsID(ref)
	return map[string]interface{}{
		"Ref": id,
	}
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
	contract.Assert(stack != nil)
	glog.V(4).Infof("Generating stack service templates: stack=%v private=%v public=%v",
		stack.Name, len(stack.Services.Private), len(stack.Services.Public))

	privates := make(cfResources)
	publics := make(cfResources)
	defer glog.V(5).Infof("Generated stack service templates: stack=%v private=%v public=%v",
		stack.Name, len(privates), len(publics))

	for _, name := range ast.StableServices(stack.Services.Private) {
		for nm, r := range c.genServiceTemplate(comp, stack, stack.Services.Private[name]) {
			privates[nm] = r
		}
	}

	for _, name := range ast.StableServices(stack.Services.Public) {
		for nm, r := range c.genServiceTemplate(comp, stack, stack.Services.Public[name]) {
			publics[nm] = r
		}
	}

	return privates, publics
}

// genServiceTemplate creates a CloudFormation resource for a single service.
func (c *awsCloud) genServiceTemplate(comp core.Compiland, stack *ast.Stack, svc *ast.Service) cfResources {
	// Instantiate and textually include the stack into our current template.
	// TODO: consider an option where a Stack can become a distinct CloudFormation Stack, and then reference it by
	//     name.  This would be a terrible default, because we'd end up with dozens of CloudFormation Stacks for even
	//     the simplest of Mu Stacks.  Especially because many Mu Stacks are single-Service.  Perhaps we could come
	//     up with some clever default, like multi-Service Mu Stacks map to CloudFormation Stacks, and single-Service
	//     ones don't, however I'm not yet convinced this is the right path.  So, for now, we keep it simple.
	contract.Assert(svc.BoundType != nil)
	glog.V(4).Infof("Generating service template: svc=%v type=%v", svc.Name, svc.BoundType.Name)

	if svc.BoundType.Intrinsic {
		// For intrinsics, generate code for those that we understand; for all others, issue an error.
		switch svc.BoundType.Name {
		case cfIntrinsicName:
			intrin := asCFIntrinsic(svc)
			return c.genCFIntrinsicServiceTemplate(comp, stack, intrin)
		default:
			c.Diag().Errorf(errors.ErrorUnrecognizedIntrinsic.At(stack), svc.BoundType.Name)
		}
	}

	// For everything else, go through the usual template generation.
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

// genCFIntrinsicServiceTemplate simply creates a CF resource out of the provided template.  This will have already been
// typechecked, etc., during earlier phases of the compiler; extract it into a strong wrapper.
func (c *awsCloud) genCFIntrinsicServiceTemplate(comp core.Compiland, stack *ast.Stack, cf *cfIntrinsic) cfResources {
	resProps := make(cfResourceProperties)

	// Map the properties requested.
	for _, to := range ast.StableStringStringMap(cf.Properties) {
		from := cf.Properties[to]
		if p, has := stack.BoundPropertyValues[from]; has {
			resProps[to] = conv.ToValue(p)
		} else {
			// It's ok if there is no bound property for this; that just means the caller didn't supply a value, which
			// is totally legal for optional properties.  But at least make sure the property refers to a valid property
			// name, otherwise this could be a mistake (mispelled name, etc).
			if _, has := stack.Properties[from]; !has {
				c.Diag().Errorf(ErrorPropertyNotFound.At(stack), from)
			}
		}
	}

	// If there are any "extra" properties, merge them in with the existing map.
	for _, exname := range ast.StableKeys(cf.ExtraProperties) {
		v := cf.ExtraProperties[exname]
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

	// If there are any explicit dependencies listed, we need to fish them out and add them.
	var resDeps []cfLogicalID
	for _, d := range cf.DependsOn {
		resDeps = append(resDeps, c.genResourceDependsID(d))
	}

	// Finally, generate an ID from the service's name, and return the result.
	id := c.genResourceID(stack, cf.Service)
	return cfResources{
		id: cfResource{
			Type:       cfResourceType(cf.Resource),
			Properties: cfResourceProperties(resProps),
			DependsOn:  resDeps,
		},
	}
}
