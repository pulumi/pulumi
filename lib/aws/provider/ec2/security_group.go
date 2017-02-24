// Copyright 2016 Marapongo, Inc. All rights reserved.

package ec2

import (
	"errors"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
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

// Name names a given resource.  Sometimes this will be assigned by a developer, and so the provider
// simply fetches it from the property bag; other times, the provider will assign this based on its own algorithm.
// In any case, resources with the same name must be safe to use interchangeably with one another.
func (p *securityGroupProvider) Name(ctx context.Context, req *murpc.NameRequest) (*murpc.NameResponse, error) {
	return nil, nil // use the AWS provider default name
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
	fmt.Fprintf(os.Stdout, "Creating EC2 security group")
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
	fmt.Fprintf(os.Stdout, "EC2 security group created: %v; waiting for it to become active\n", *id)

	// Don't proceed until the security group exists.
	if err = p.waitForSecurityGroupState(id, true); err != nil {
		return nil, err
	}

	// Authorize ingress rules if any exist.
	if secgrp.Ingress != nil {
		fmt.Fprintf(os.Stdout, "Authorizing %v security group ingress rules\n", len(*secgrp.Ingress))
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
		fmt.Fprintf(os.Stdout, "Authorizing %v security group egress rules\n", len(*secgrp.Egress))
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

	// First, perform the deletion.
	id := aws.String(req.GetId())
	fmt.Fprintf(os.Stdout, "Terminating EC2 SecurityGroup '%v'\n", *id)
	delete := &ec2.DeleteSecurityGroupInput{GroupId: id}
	if _, err := p.ctx.EC2().DeleteSecurityGroup(delete); err != nil {
		return nil, err
	}

	fmt.Fprintf(os.Stdout, "EC2 Security Group delete request submitted; waiting for it to terminate\n")

	// Don't proceed until the security group exists.
	if err := p.waitForSecurityGroupState(id, false); err != nil {
		return nil, err
	}

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

func (p *securityGroupProvider) waitForSecurityGroupState(id *string, exist bool) error {
	succ, err := awsctx.RetryUntil(
		p.ctx,
		func() (bool, error) {
			req := &ec2.DescribeSecurityGroupsInput{GroupIds: []*string{id}}
			missing := true
			res, err := p.ctx.EC2().DescribeSecurityGroups(req)
			if err != nil {
				if !isSecurityGroupNotExistErr(err) {
					return false, err // quit and propagate the error
				}
			} else if res != nil && len(res.SecurityGroups) > 0 {
				contract.Assert(len(res.SecurityGroups) == 1)
				contract.Assert(*res.SecurityGroups[0].GroupId == *id)
				missing = false // we found one
			}

			if missing {
				// If missing and exist==true, keep retrying; else, we're good.
				return !exist, nil
			} else {
				// If not missing and exist==true, we're good; else, keep retrying.
				return exist, nil
			}
		},
	)
	if err != nil {
		return err
	} else if !succ {
		var reason string
		if exist {
			reason = "become active"
		} else {
			reason = "terminate"
		}
		return fmt.Errorf("EC2 security group '%v' did not %v", *id, reason)
	}
	return nil
}

func isSecurityGroupNotExistErr(err error) bool {
	if erraws, iserraws := err.(awserr.Error); iserraws {
		if erraws.Code() == "InvalidGroup.NotFound" {
			// The specified security group does not eixst; this error can occur because the ID of a recently created
			// security group has not propagated through the system.
			return true
		}
		if erraws.Code() == "InvalidSecurityGroupID.NotFound" {
			// The specified security group does not exist; if you are creating a network interface, ensure that
			// you specify a VPC security group, and not an EC2-Classic security group.
			return true
		}
	}
	return false
}
