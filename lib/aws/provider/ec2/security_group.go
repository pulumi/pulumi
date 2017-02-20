// Copyright 2016 Marapongo, Inc. All rights reserved.

package ec2

import (
	"errors"

	"github.com/aws/aws-sdk-go/service/ec2"
	pbempty "github.com/golang/protobuf/ptypes/empty"
	"github.com/marapongo/mu/pkg/resource"
	"github.com/marapongo/mu/pkg/tokens"
	"github.com/marapongo/mu/pkg/util/contract"
	"github.com/marapongo/mu/sdk/go/pkg/murpc"
	"golang.org/x/net/context"

	"github.com/marapongo/mu/lib/aws/provider/awsctx"
)

const SecurityGroup = tokens.Type("aws:ec2/securityGroup:SecurityGroup")

// NewSecurityGroupProvider creates a provider that handles EC2 security group operations.
func NewSecurityGroupProvider(ctx *awsctx.Context) murpc.ResourceProviderServer {
	return &securityGroupProvider{ctx}
}

type securityGroupProvider struct {
	ctx *awsctx.Context
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transacational").
func (p *securityGroupProvider) Create(ctx context.Context, req *murpc.CreateRequest) (*murpc.CreateResponse, error) {
	contract.Assert(req.GetType() == string(SecurityGroup))
	props := resource.UnmarshalProperties(req.GetProperties())

	// Read in the properties given by the request, validating as we go; if any fail, reject the request.
	// TODO: this is a good example of a "benign" (StateOK) error; handle it accordingly.
	secgrp, err := newSecurityGroup(props, true)
	if err != nil {
		return nil, err
	}

	// Make the security group creation parameters.
	// TODO: the name needs to be figured out; CloudFormation doesn't expose it, presumably due to its requirement to
	//     be unique.  I think we can use the moniker here, but that isn't necessarily stable.  UUID?
	create := &ec2.CreateSecurityGroupInput{
		GroupName:   &secgrp.Description,
		Description: &secgrp.Description,
		VpcId:       secgrp.VPCID,
	}

	// Now go ahead and perform the action.
	result, err := p.ctx.EC2().CreateSecurityGroup(create)
	if err != nil {
		return nil, err
	}
	contract.Assert(result != nil)
	contract.Assert(result.GroupId != nil)

	// TODO: memoize the ID.
	// TODO: wait for the group to finish spinning up.
	// TODO: create the ingress/egress rules.

	return &murpc.CreateResponse{
		Id: *result.GroupId,
	}, nil
}

// Read reads the instance state identified by ID, returning a populated resource object, or an error if not found.
func (p *securityGroupProvider) Read(ctx context.Context, req *murpc.ReadRequest) (*murpc.ReadResponse, error) {
	contract.Assert(req.GetType() == string(SecurityGroup))
	return nil, errors.New("Not yet implemented")
}

// Update updates an existing resource with new values.  Only those values in the provided property bag are updated
// to new values.  The resource ID is returned and may be different if the resource had to be recreated.
func (p *securityGroupProvider) Update(ctx context.Context, req *murpc.UpdateRequest) (*murpc.UpdateResponse, error) {
	contract.Assert(req.GetType() == string(SecurityGroup))
	return nil, errors.New("Not yet implemented")
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
func (p *securityGroupProvider) Delete(ctx context.Context, req *murpc.DeleteRequest) (*pbempty.Empty, error) {
	contract.Assert(req.GetType() == string(SecurityGroup))
	return nil, errors.New("Not yet implemented")
}

// securityGroup represents the state associated with a security group.
type securityGroup struct {
	Description string                      // description of the security group.
	VPCID       *string                     // the VPC in which this security group resides.
	Egress      *[]securityGroupEgressRule  // a list of security group egress rules.
	Ingress     *[]securityGroupIngressRule // a list of security group ingress rules.
}

// newSecurityGroup creates a new instance bag of state, validating required properties if asked to do so.
func newSecurityGroup(m resource.PropertyMap, req bool) (*securityGroup, error) {
	description, err := m.ReqStringOrErr("groupDescription")
	if err != nil && (req || !resource.IsReqError(err)) {
		return nil, err
	}

	// TODO: validate other aspects of the parameters; for instance, ensure that the description is < 255 characters,
	//     etc.  Furthermore, consider doing this in a pass before performing *any* actions (and during planning), so
	//     that we can hoist failures to before even trying to execute a plan (when such errors are more costly).

	vpcID, err := m.OptStringOrErr("vpc")
	if err != nil {
		return nil, err
	}

	var egress *[]securityGroupEgressRule
	egressArray, err := m.OptObjectArrayOrErr("securityGroupEgress")
	if err != nil {
		return nil, err
	} else if egressArray != nil {
		var rules []securityGroupEgressRule
		for _, rule := range *egressArray {
			sger, err := newSecurityGroupEgressRule(rule, req)
			if err != nil {
				return nil, err
			}
			rules = append(rules, *sger)
		}
		egress = &rules
	}

	var ingress *[]securityGroupIngressRule
	ingressArray, err := m.OptObjectArrayOrErr("securityGroupIngress")
	if err != nil {
		return nil, err
	} else if ingressArray != nil {
		var rules []securityGroupIngressRule
		for _, rule := range *ingressArray {
			sgir, err := newSecurityGroupIngressRule(rule, req)
			if err != nil {
				return nil, err
			}
			rules = append(rules, *sgir)
		}
		ingress = &rules
	}

	return &securityGroup{
		Description: description,
		VPCID:       vpcID,
		Egress:      egress,
		Ingress:     ingress,
	}, nil
}

// securityGroupRule represents the state associated with a security group rule.
type securityGroupRule struct {
	IPProtocol string  // an IP protocol name or number.
	CIDRIP     *string // specifies a CIDR range.
	FromPort   *int64  // the start of port range for the TCP/UDP protocols, or an ICMP type number.
	ToPort     *int64  // the end of port range for the TCP/UDP protocols, or an ICMP code.
}

// newSecurityGroupRule creates a new instance bag of state, validating required properties if asked to do so.
func newSecurityGroupRule(m resource.PropertyMap, req bool) (*securityGroupRule, error) {
	ipProtocol, err := m.ReqStringOrErr("ipProtocol")
	if err != nil && (req || !resource.IsReqError(err)) {
		return nil, err
	}
	cidrIP, err := m.OptStringOrErr("cidrIp")
	if err != nil {
		return nil, err
	}

	var fromPort *int64
	fromPortF, err := m.OptNumberOrErr("fromPort")
	if err != nil {
		return nil, err
	} else {
		fromPortI := int64(*fromPortF)
		fromPort = &fromPortI
	}

	var toPort *int64
	toPortF, err := m.OptNumberOrErr("toPort")
	if err != nil {
		return nil, err
	} else {
		toPortI := int64(*toPortF)
		toPort = &toPortI
	}

	return &securityGroupRule{
		IPProtocol: ipProtocol,
		CIDRIP:     cidrIP,
		FromPort:   fromPort,
		ToPort:     toPort,
	}, nil
}

// securityGroupEgressRule represents the state associated with a security group egress rule.
type securityGroupEgressRule struct {
	securityGroupRule
	DestinationPrefixListID    *string // the AWS service prefix of an Amazon VPC endpoint.
	DestinationSecurityGroupID *string // specifies the destination Amazon VPC security group.
}

// newSecurityEgressGroupRule creates a new instance bag of state, validating required properties if asked to do so.
func newSecurityGroupEgressRule(m resource.PropertyMap, req bool) (*securityGroupEgressRule, error) {
	rule, err := newSecurityGroupRule(m, req)
	if err != nil && (req || !resource.IsReqError(err)) {
		return nil, err
	}
	destPrefixListID, err := m.OptStringOrErr("destinationPrefixListId")
	if err != nil {
		return nil, err
	}
	destSecurityGroupID, err := m.OptStringOrErr("destinationSecurityGroup")
	if err != nil {
		return nil, err
	}
	return &securityGroupEgressRule{
		securityGroupRule:          *rule,
		DestinationPrefixListID:    destPrefixListID,
		DestinationSecurityGroupID: destSecurityGroupID,
	}, nil
}

// securityGroupIngressRule represents the state associated with a security group ingress rule.
type securityGroupIngressRule struct {
	securityGroupRule
	SourceSecurityGroupID      *string // the ID of a security group to allow access (for VPC groups only).
	SourceSecurityGroupName    *string // the name of a security group to allow access (for non-VPC groups only).
	SourceSecurityGroupOwnerID *string // the account ID of the owner of the group sepcified by the name, if any.
}

// newSecurityIngressGroupRule creates a new instance bag of state, validating required properties if asked to do so.
func newSecurityGroupIngressRule(m resource.PropertyMap, req bool) (*securityGroupIngressRule, error) {
	rule, err := newSecurityGroupRule(m, req)
	if err != nil && (req || !resource.IsReqError(err)) {
		return nil, err
	}
	srcSecurityGroupID, err := m.OptStringOrErr("sourceSecurityGroup")
	if err != nil {
		return nil, err
	}
	srcSecurityGroupName, err := m.OptStringOrErr("sourceSecurityGroupName")
	if err != nil {
		return nil, err
	}
	srcSecurityGroupOwnerID, err := m.OptStringOrErr("sourceSecurityGroupOwnerId")
	if err != nil {
		return nil, err
	}
	return &securityGroupIngressRule{
		securityGroupRule:          *rule,
		SourceSecurityGroupID:      srcSecurityGroupID,
		SourceSecurityGroupName:    srcSecurityGroupName,
		SourceSecurityGroupOwnerID: srcSecurityGroupOwnerID,
	}, nil
}
