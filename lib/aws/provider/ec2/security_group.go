// Copyright 2016 Marapongo, Inc. All rights reserved.

package ec2

import (
	"errors"

	"github.com/aws/aws-sdk-go/aws"
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
	id := result.GroupId
	contract.Assert(id != nil)

	// TODO: wait for the group to finish spinning up.

	// Authorize ingress rules if any exist.
	if secgrp.Ingress != nil {
		for _, ingress := range *secgrp.Ingress {
			authin := &ec2.AuthorizeSecurityGroupIngressInput{
				GroupId:    id,
				IpProtocol: aws.String(ingress.IPProtocol),
				CidrIp:     ingress.CIDRIP,
				FromPort:   ingress.FromPort,
				ToPort:     ingress.ToPort,
			}
			if _, err := p.ctx.EC2().AuthorizeSecurityGroupIngress(authin); err != nil {
				return nil, err
			}
		}
	}

	// Authorize egress rules if any exist.
	if secgrp.Egress != nil {
		for _, egress := range *secgrp.Egress {
			authout := &ec2.AuthorizeSecurityGroupEgressInput{
				GroupId:    id,
				IpProtocol: aws.String(egress.IPProtocol),
				CidrIp:     egress.CIDRIP,
				FromPort:   egress.FromPort,
				ToPort:     egress.ToPort,
			}
			if _, err := p.ctx.EC2().AuthorizeSecurityGroupEgress(authout); err != nil {
				return nil, err
			}
		}
	}

	return &murpc.CreateResponse{
		Id: *id,
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
	delete := &ec2.DeleteSecurityGroupInput{
		GroupId: aws.String(req.GetId()),
	}
	if _, err := p.ctx.EC2().DeleteSecurityGroup(delete); err != nil {
		return nil, err
	}
	// TODO: wait for termination to complete.
	return &pbempty.Empty{}, nil
}

// securityGroup represents the state associated with a security group.
type securityGroup struct {
	Description string               // description of the security group.
	VPCID       *string              // the VPC in which this security group resides.
	Egress      *[]securityGroupRule // a list of security group egress rules.
	Ingress     *[]securityGroupRule // a list of security group ingress rules.
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

	var egress *[]securityGroupRule
	egressArray, err := m.OptObjectArrayOrErr("securityGroupEgress")
	if err != nil {
		return nil, err
	} else if egressArray != nil {
		var rules []securityGroupRule
		for _, rule := range *egressArray {
			secrule, err := newSecurityGroupRule(rule, req)
			if err != nil {
				return nil, err
			}
			rules = append(rules, *secrule)
		}
		egress = &rules
	}

	var ingress *[]securityGroupRule
	ingressArray, err := m.OptObjectArrayOrErr("securityGroupIngress")
	if err != nil {
		return nil, err
	} else if ingressArray != nil {
		var rules []securityGroupRule
		for _, rule := range *ingressArray {
			secrule, err := newSecurityGroupRule(rule, req)
			if err != nil {
				return nil, err
			}
			rules = append(rules, *secrule)
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
