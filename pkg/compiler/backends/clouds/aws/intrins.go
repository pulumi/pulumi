// Copyright 2016 Marapongo, Inc. All rights reserved.

package aws

import (
	"github.com/marapongo/mu/pkg/ast"
	"github.com/marapongo/mu/pkg/util"
)

const namespace = "aws/x"

const cfIntrinsicName = namespace + ast.NameDelimiter + "cf"
const cfIntrinsicResource = "resource"
const cfIntrinsicDependsOn = "dependsOn"
const cfIntrinsicProperties = "properties"
const cfIntrinsicSkipProperties = "skipProperties"
const cfIntrinsicExtraProperties = "extraProperties"

// cfIntrinsic is a service with an intrinsic type allowing stacks to directly generate arbitrary CloudFormation
// templating as the output.  This forms the basic for most AWS cloud native resources.  Expansion of this type happens
// after Mu templates have been expanded, allowing stack properties, target environments, and so on, to be leveraged in
// the way these templates are generated.
type cfIntrinsic struct {
	*ast.Service
	Resource        ast.StringLiteral
	DependsOn       ast.ServiceListLiteral
	Properties      ast.StringListLiteral
	SkipProperties  ast.StringListLiteral
	ExtraProperties ast.StringMapLiteral
}

// AsCFIntrinsic converts a given service to a CloudFormationService, validating it as we go.
func asCFIntrinsic(svc *ast.Service) *cfIntrinsic {
	util.AssertM(svc.BoundType.Name == cfIntrinsicName, "asCFIntrinsic expects a bound CF service type")

	res := &cfIntrinsic{
		Service: svc,
	}

	if r, ok := svc.BoundProperties[cfIntrinsicResource]; ok {
		res.Resource, ok = r.(ast.StringLiteral)
		util.Assert(ok)
	} else {
		util.FailMF("Expected a required 'resource' property")
	}
	if do, ok := svc.BoundProperties[cfIntrinsicDependsOn]; ok {
		res.DependsOn, ok = do.(ast.ServiceListLiteral)
		util.Assert(ok)
	}
	if props, ok := svc.BoundProperties[cfIntrinsicProperties]; ok {
		res.Properties, ok = props.(ast.StringListLiteral)
		util.Assert(ok)
	}
	if skips, ok := svc.BoundProperties[cfIntrinsicSkipProperties]; ok {
		res.SkipProperties, ok = skips.(ast.StringListLiteral)
		util.Assert(ok)
	}
	if extras, ok := svc.BoundProperties[cfIntrinsicExtraProperties]; ok {
		res.ExtraProperties, ok = extras.(ast.StringMapLiteral)
		util.Assert(ok)
	}

	return res
}
