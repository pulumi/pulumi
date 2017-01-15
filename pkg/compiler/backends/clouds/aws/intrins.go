// Copyright 2016 Marapongo, Inc. All rights reserved.

package aws

import (
	"github.com/marapongo/mu/pkg/ast"
	"github.com/marapongo/mu/pkg/ast/conv"
	"github.com/marapongo/mu/pkg/util/contract"
)

const namespace = "aws/x"

const cfIntrinsicName = namespace + ast.NameDelimiter + "cf"
const cfIntrinsicResource = "resource"
const cfIntrinsicDependsOn = "dependsOn"
const cfIntrinsicProperties = "properties"
const cfIntrinsicExtraProperties = "extraProperties"

// cfIntrinsic is a service with an intrinsic type allowing stacks to directly generate arbitrary CloudFormation
// templating as the output.  This forms the basic for most AWS cloud native resources.  Expansion of this type happens
// after Mu templates have been expanded, allowing stack properties, target environments, and so on, to be leveraged in
// the way these templates are generated.
type cfIntrinsic struct {
	*ast.Service
	Resource        string
	DependsOn       []*ast.ServiceRef
	Properties      map[string]string
	ExtraProperties map[string]interface{}
}

// AsCFIntrinsic converts a given service to a CloudFormationService, validating it as we go.
func asCFIntrinsic(svc *ast.Service) *cfIntrinsic {
	contract.AssertM(svc.BoundType.Name == cfIntrinsicName, "asCFIntrinsic expects a bound CF service type")

	res := &cfIntrinsic{
		Service: svc,
	}

	if r, ok := svc.BoundProperties[cfIntrinsicResource]; ok {
		res.Resource, ok = conv.ToString(r)
		contract.Assert(ok)
	} else {
		contract.FailMF("Expected a required 'resource' property")
	}
	if do, ok := svc.BoundProperties[cfIntrinsicDependsOn]; ok {
		res.DependsOn, ok = conv.ToServiceArray(do)
		contract.Assert(ok)
	}
	if props, ok := svc.BoundProperties[cfIntrinsicProperties]; ok {
		res.Properties, ok = conv.ToStringStringMap(props)
		contract.Assert(ok)
	}
	if extras, ok := svc.BoundProperties[cfIntrinsicExtraProperties]; ok {
		res.ExtraProperties, ok = conv.ToStringMap(extras)
		contract.Assert(ok)
	}

	return res
}
