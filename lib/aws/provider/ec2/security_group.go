// Copyright 2017 Pulumi, Inc. All rights reserved.

package ec2

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"reflect"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	pbempty "github.com/golang/protobuf/ptypes/empty"
	pbstruct "github.com/golang/protobuf/ptypes/struct"
	"github.com/pulumi/coconut/pkg/resource"
	"github.com/pulumi/coconut/pkg/tokens"
	"github.com/pulumi/coconut/pkg/util/contract"
	"github.com/pulumi/coconut/pkg/util/mapper"
	"github.com/pulumi/coconut/sdk/go/pkg/cocorpc"
	"golang.org/x/net/context"

	"github.com/pulumi/coconut/lib/aws/provider/awsctx"
)

const SecurityGroup = tokens.Type("aws:ec2/securityGroup:SecurityGroup")

// NewSecurityGroupProvider creates a provider that handles EC2 security group operations.
func NewSecurityGroupProvider(ctx *awsctx.Context) cocorpc.ResourceProviderServer {
	return &sgProvider{ctx}
}

type sgProvider struct {
	ctx *awsctx.Context
}

// Check validates that the given property bag is valid for a resource of the given type.
func (p *sgProvider) Check(ctx context.Context, req *cocorpc.CheckRequest) (*cocorpc.CheckResponse, error) {
	// Read in the properties, create and validate a new group, and return the failures (if any).
	contract.Assert(req.GetType() == string(SecurityGroup))
	_, _, result := unmarshalSecurityGroup(req.GetProperties())
	return resource.NewCheckResponse(result), nil
}

// Name names a given resource.  Sometimes this will be assigned by a developer, and so the provider
// simply fetches it from the property bag; other times, the provider will assign this based on its own algorithm.
// In any case, resources with the same name must be safe to use interchangeably with one another.
func (p *sgProvider) Name(ctx context.Context, req *cocorpc.NameRequest) (*cocorpc.NameResponse, error) {
	return nil, nil // use the AWS provider default name
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transacational").
func (p *sgProvider) Create(ctx context.Context, req *cocorpc.CreateRequest) (*cocorpc.CreateResponse, error) {
	contract.Assert(req.GetType() == string(SecurityGroup))

	// Read in the properties given by the request, validating as we go; if any fail, reject the request.
	secgrp, _, decerr := unmarshalSecurityGroup(req.GetProperties())
	if decerr != nil {
		// TODO: this is a good example of a "benign" (StateOK) error; handle it accordingly.
		return nil, decerr
	}

	// Make the security group creation parameters.  The name of the group is auto-generated using a random hash so
	// that we can avoid conflicts with existing similarly named groups.  For readability, we prefix the real name.
	name := resource.NewUniqueHex(secgrp.Name+"-", maxSecurityGroupName, sha1.Size)
	fmt.Printf("Creating EC2 security group with name '%v'\n", name)
	create := &ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(name),
		Description: &secgrp.Description,
		VpcId:       secgrp.VPCID.StringPtr(),
	}

	// Now go ahead and perform the action.
	result, err := p.ctx.EC2().CreateSecurityGroup(create)
	if err != nil {
		return nil, err
	}
	contract.Assert(result != nil)
	id := result.GroupId
	contract.Assert(id != nil)
	fmt.Printf("EC2 security group created: %v; waiting for it to become active\n", *id)

	// Don't proceed until the security group exists.
	if err = p.waitForSecurityGroupState(id, true); err != nil {
		return nil, err
	}

	// Authorize ingress rules if any exist.
	if len(secgrp.Ingress) > 0 {
		fmt.Printf("Authorizing %v security group ingress (inbound) rules\n", len(secgrp.Ingress))
		for _, ingress := range secgrp.Ingress {
			if err := p.createSecurityGroupIngressRule(id, ingress); err != nil {
				return nil, err
			}
		}
	}

	// Authorize egress rules if any exist.
	if len(secgrp.Egress) > 0 {
		fmt.Printf("Authorizing %v security group egress (outbound) rules\n", len(secgrp.Egress))
		for _, egress := range secgrp.Egress {
			if err := p.createSecurityGroupEgressRule(id, egress); err != nil {
				return nil, err
			}
		}
	}

	return &cocorpc.CreateResponse{
		Id: *id,
	}, nil
}

// Read reads the instance state identified by ID, returning a populated resource object, or an error if not found.
func (p *sgProvider) Read(ctx context.Context, req *cocorpc.ReadRequest) (*cocorpc.ReadResponse, error) {
	contract.Assert(req.GetType() == string(SecurityGroup))
	return nil, errors.New("Not yet implemented")
}

// Update updates an existing resource with new values.  Only those values in the provided property bag are updated
// to new values.  The resource ID is returned and may be different if the resource had to be recreated.
func (p *sgProvider) Update(ctx context.Context, req *cocorpc.UpdateRequest) (*pbempty.Empty, error) {
	contract.Assert(req.GetType() == string(SecurityGroup))

	// Provided it's okay, unmarshal, validate, and diff the properties.
	id := req.GetId()
	oldgrp, newgrp, diff, replaces, err := unmarshalSecurityGroupChange(req.GetOlds(), req.GetNews())
	if err != nil {
		return nil, err
	}

	// If this was a replacement, the UpdateImpact routine should have rejected it.
	if len(replaces) > 0 {
		return nil, errors.New("this update requires a resource replacement")
	}

	// If only the ingress and/or egress rules changed, we can incrementally apply the updates.
	gresses := []struct {
		key    resource.PropertyKey
		olds   []securityGroupRule
		news   []securityGroupRule
		create func(*string, securityGroupRule) error
		delete func(*string, securityGroupRule) error
	}{
		{
			securityGroupIngress,
			newgrp.Ingress,
			oldgrp.Ingress,
			p.createSecurityGroupIngressRule,
			p.deleteSecurityGroupIngressRule,
		},
		{
			securityGroupEgress,
			newgrp.Egress,
			oldgrp.Egress,
			p.createSecurityGroupEgressRule,
			p.deleteSecurityGroupEgressRule,
		},
	}
	for _, gress := range gresses {
		if diff.Changed(gress.key) {
			// First accumulate the diffs.
			var creates []securityGroupRule
			var deletes []securityGroupRule
			if diff.Added(gress.key) {
				contract.Assert(len(gress.news) > 0)
				for _, rule := range gress.news {
					creates = append(creates, rule)
				}
			} else if diff.Deleted(gress.key) {
				contract.Assert(len(gress.olds) > 0)
				for _, rule := range gress.olds {
					deletes = append(deletes, rule)
				}
			} else if diff.Updated(gress.key) {
				update := diff.Updates[gress.key]
				contract.Assert(update.Array != nil)
				for _, add := range update.Array.Adds {
					contract.Assert(add.IsObject())
					var rule securityGroupRule
					if err := mapper.MapIU(add.ObjectValue().Mappable(), &rule); err != nil {
						return nil, err
					}
					creates = append(creates, rule)
				}
				for _, delete := range update.Array.Deletes {
					contract.Assert(delete.IsObject())
					var rule securityGroupRule
					if err := mapper.MapIU(delete.ObjectValue().Mappable(), &rule); err != nil {
						return nil, err
					}
					deletes = append(deletes, rule)
				}
				for _, change := range update.Array.Updates {
					// We can't update individual fields of a rule; simply delete and recreate.
					var before securityGroupRule
					contract.Assert(change.Old.IsObject())
					if err := mapper.MapIU(change.Old.ObjectValue().Mappable(), &before); err != nil {
						return nil, err
					}
					deletes = append(deletes, before)
					var after securityGroupRule
					contract.Assert(change.New.IsObject())
					if err := mapper.MapIU(change.New.ObjectValue().Mappable(), &after); err != nil {
						return nil, err
					}
					creates = append(creates, after)
				}
			}

			// And now actually perform the create and delete operations.
			for _, delete := range deletes {
				if err := p.deleteSecurityGroupIngressRule(&id, delete); err != nil {
					return nil, err
				}
			}
			for _, create := range creates {
				if err := p.createSecurityGroupIngressRule(&id, create); err != nil {
					return nil, err
				}
			}
		}
	}

	return &pbempty.Empty{}, nil
}

// UpdateImpact checks what impacts a hypothetical update will have on the resource's properties.
func (p *sgProvider) UpdateImpact(
	ctx context.Context, req *cocorpc.UpdateRequest) (*cocorpc.UpdateImpactResponse, error) {
	contract.Assert(req.GetType() == string(SecurityGroup))
	// First unmarshal and validate the properties.
	_, _, _, replaces, err := unmarshalSecurityGroupChange(req.GetOlds(), req.GetNews())
	if err != nil {
		return nil, err
	}
	return &cocorpc.UpdateImpactResponse{
		Replaces: replaces,
		// TODO: serialize the other properties that will be updated.
	}, nil
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
func (p *sgProvider) Delete(ctx context.Context, req *cocorpc.DeleteRequest) (*pbempty.Empty, error) {
	contract.Assert(req.GetType() == string(SecurityGroup))

	// First, perform the deletion.
	id := aws.String(req.GetId())
	fmt.Printf("Terminating EC2 SecurityGroup '%v'\n", *id)
	delete := &ec2.DeleteSecurityGroupInput{GroupId: id}
	if _, err := p.ctx.EC2().DeleteSecurityGroup(delete); err != nil {
		return nil, err
	}

	fmt.Printf("EC2 Security Group delete request submitted; waiting for it to terminate\n")

	// Don't proceed until the security group exists.
	if err := p.waitForSecurityGroupState(id, false); err != nil {
		return nil, err
	}

	return &pbempty.Empty{}, nil
}

// securityGroup represents the state associated with a security group.
type securityGroup struct {
	Name        string              `json:"name"`                           // the security group's unique name.
	Description string              `json:"groupDescription"`               // description of the security group.
	VPCID       *resource.ID        `json:"vpc,omitempty"`                  // the security group's VPC..
	Egress      []securityGroupRule `json:"securityGroupEgress,omitempty"`  // a list of security group egress rules.
	Ingress     []securityGroupRule `json:"securityGroupIngress,omitempty"` // a list of security group ingress rules.
}

// constants for all of the security group property names.
const (
	securityGroupName        = "name"
	securityGroupDescription = "groupDescription"
	securityGroupVPCID       = "vpc"
	securityGroupEgress      = "securityGroupEgress"
	securityGroupIngress     = "securityGroupIngress"
)

// constants for the various security group limits.
const (
	maxSecurityGroupName        = 255
	maxSecurityGroupDescription = 255
)

// unmarshalSecurityGroup decodes and validates a security group.
func unmarshalSecurityGroup(v *pbstruct.Struct) (securityGroup, resource.PropertyMap, mapper.DecodeError) {
	var secgrp securityGroup
	props := resource.UnmarshalProperties(v)
	result := mapper.MapIU(props.Mappable(), &secgrp)
	if len(secgrp.Description) > maxSecurityGroupDescription {
		if result == nil {
			result = mapper.NewDecodeErr(nil)
		}
		result.AddFailure(
			mapper.NewFieldErr(reflect.TypeOf(secgrp), securityGroupDescription,
				fmt.Errorf("exceeded maximum length of %v", maxSecurityGroupDescription)))
	}
	return secgrp, props, result
}

// unmarshalSecurityGroupChange unmarshals old and new properties, diffs them and checks whether resource
// replacement is necessary.  If an error occurs, the returned error is non-nil.
func unmarshalSecurityGroupChange(olds *pbstruct.Struct,
	news *pbstruct.Struct) (*securityGroup, *securityGroup, *resource.ObjectDiff, []string, error) {
	// Deserialize the old/new properties and validate them before bothering to diff them.
	oldgrp, oldprops, err := unmarshalSecurityGroup(olds)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	newgrp, newprops, err := unmarshalSecurityGroup(news)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	// Now diff the properties to determine whether this must be recreated.
	var replaces []string
	diff := oldprops.Diff(newprops)
	if diff.Changed(securityGroupName) {
		replaces = append(replaces, securityGroupName)
	}
	if diff.Changed(securityGroupDescription) {
		replaces = append(replaces, securityGroupDescription)
	}
	if diff.Changed(securityGroupVPCID) {
		replaces = append(replaces, securityGroupVPCID)
	}
	return &oldgrp, &newgrp, diff, replaces, nil
}

// securityGroupRule represents the state associated with a security group rule.
type securityGroupRule struct {
	IPProtocol string  `json:"ipProtocol"`         // an IP protocol name or number.
	CIDRIP     *string `json:"cidrIp,omitempty"`   // specifies a CIDR range.
	FromPort   *int64  `json:"fromPort,omitempty"` // the start of port range for TCP/UDP or an ICMP code.
	ToPort     *int64  `json:"toPort,omitempty"`   // the end of port range for TCP/UDP or an ICMP code.
}

// constants for all of the security group rule property names.
const (
	securityGroupRuleIPProtocol = "ipProtocol"
	securityGroupRuleCIDRIP     = "cidrIp"
	securityGroupRuleFromPort   = "fromPort"
	securityGroupRuleToPort     = "toPort"
)

func (p *sgProvider) createSecurityGroupIngressRule(groupID *string, ingress securityGroupRule) error {
	// First print a little status to stdout.
	fmt.Printf("Authorizing security group ingress (inbound) rule: IPProtocol=%v", ingress.IPProtocol)
	if ingress.CIDRIP != nil {
		fmt.Printf(", CIDRIP=%v", *ingress.CIDRIP)
	}
	if ingress.FromPort != nil {
		fmt.Printf(", FromPort=%v", *ingress.FromPort)
	}
	if ingress.ToPort != nil {
		fmt.Printf(", ToPort=%v", *ingress.ToPort)
	}
	fmt.Printf("\n")

	// Now do it.
	authin := &ec2.AuthorizeSecurityGroupIngressInput{
		GroupId:    groupID,
		IpProtocol: aws.String(ingress.IPProtocol),
		CidrIp:     ingress.CIDRIP,
		FromPort:   ingress.FromPort,
		ToPort:     ingress.ToPort,
	}
	if _, err := p.ctx.EC2().AuthorizeSecurityGroupIngress(authin); err != nil {
		return err
	}
	return nil
}

func (p *sgProvider) deleteSecurityGroupIngressRule(groupID *string, ingress securityGroupRule) error {
	// First print a little status to stdout.
	fmt.Printf("Revoking security group ingress (inbound) rule: IPProtocol=%v", ingress.IPProtocol)
	if ingress.CIDRIP != nil {
		fmt.Printf(", CIDRIP=%v", *ingress.CIDRIP)
	}
	if ingress.FromPort != nil {
		fmt.Printf(", FromPort=%v", *ingress.FromPort)
	}
	if ingress.ToPort != nil {
		fmt.Printf(", ToPort=%v", *ingress.ToPort)
	}
	fmt.Printf("\n")

	// Now do it.
	revokin := &ec2.RevokeSecurityGroupIngressInput{
		GroupId:    groupID,
		IpProtocol: aws.String(ingress.IPProtocol),
		CidrIp:     ingress.CIDRIP,
		FromPort:   ingress.FromPort,
		ToPort:     ingress.ToPort,
	}
	if _, err := p.ctx.EC2().RevokeSecurityGroupIngress(revokin); err != nil {
		return err
	}
	return nil
}

func (p *sgProvider) createSecurityGroupEgressRule(groupID *string, egress securityGroupRule) error {
	// First print a little status to stdout.
	fmt.Printf("Authorizing security group egress (outbound) rule: IPProtocol=%v", egress.IPProtocol)
	if egress.CIDRIP != nil {
		fmt.Printf(", CIDRIP=%v", *egress.CIDRIP)
	}
	if egress.FromPort != nil {
		fmt.Printf(", FromPort=%v", *egress.FromPort)
	}
	if egress.ToPort != nil {
		fmt.Printf(", ToPort=%v", *egress.ToPort)
	}
	fmt.Printf("\n")

	// Now do it.
	authout := &ec2.AuthorizeSecurityGroupEgressInput{
		GroupId:    groupID,
		IpProtocol: aws.String(egress.IPProtocol),
		CidrIp:     egress.CIDRIP,
		FromPort:   egress.FromPort,
		ToPort:     egress.ToPort,
	}
	if _, err := p.ctx.EC2().AuthorizeSecurityGroupEgress(authout); err != nil {
		return err

	}
	return nil
}

func (p *sgProvider) deleteSecurityGroupEgressRule(groupID *string, egress securityGroupRule) error {
	// First print a little status to stdout.
	fmt.Printf("Revoking security group egress (outbound) rule: IPProtocol=%v", egress.IPProtocol)
	if egress.CIDRIP != nil {
		fmt.Printf(", CIDRIP=%v", *egress.CIDRIP)
	}
	if egress.FromPort != nil {
		fmt.Printf(", FromPort=%v", *egress.FromPort)
	}
	if egress.ToPort != nil {
		fmt.Printf(", ToPort=%v", *egress.ToPort)
	}
	fmt.Printf("\n")

	// Now do it.
	revokout := &ec2.RevokeSecurityGroupEgressInput{
		GroupId:    groupID,
		IpProtocol: aws.String(egress.IPProtocol),
		CidrIp:     egress.CIDRIP,
		FromPort:   egress.FromPort,
		ToPort:     egress.ToPort,
	}
	if _, err := p.ctx.EC2().RevokeSecurityGroupEgress(revokout); err != nil {
		return err
	}
	return nil
}

func (p *sgProvider) waitForSecurityGroupState(id *string, exist bool) error {
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
