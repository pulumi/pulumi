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
	"github.com/pulumi/coconut/pkg/resource"
	"github.com/pulumi/coconut/pkg/util/contract"
	"github.com/pulumi/coconut/pkg/util/mapper"
	"github.com/pulumi/coconut/sdk/go/pkg/cocorpc"
	"golang.org/x/net/context"

	"github.com/pulumi/coconut/lib/aws/provider/awsctx"
	rpc "github.com/pulumi/coconut/lib/aws/rpc/ec2"
)

// constants for the various security group limits.
const (
	maxSecurityGroupName        = 255
	maxSecurityGroupDescription = 255
)

// NewSecurityGroupProvider creates a provider that handles EC2 security group operations.
func NewSecurityGroupProvider(ctx *awsctx.Context) cocorpc.ResourceProviderServer {
	ops := &sgProvider{ctx}
	return rpc.NewSecurityGroupProvider(ops)
}

type sgProvider struct {
	ctx *awsctx.Context
}

// Check validates that the given property bag is valid for a resource of the given type.
func (p *sgProvider) Check(ctx context.Context, obj *rpc.SecurityGroup) ([]mapper.FieldError, error) {
	var failures []mapper.FieldError
	if len(obj.GroupDescription) > maxSecurityGroupDescription {
		failures = append(failures,
			mapper.NewFieldErr(reflect.TypeOf(obj), rpc.SecurityGroup_GroupDescription,
				fmt.Errorf("exceeded maximum length of %v", maxSecurityGroupDescription)))
	}
	return failures, nil
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transacational").
func (p *sgProvider) Create(ctx context.Context, obj *rpc.SecurityGroup) (string, *rpc.SecurityGroupOuts, error) {
	// Make the security group creation parameters.  The name of the group is auto-generated using a random hash so
	// that we can avoid conflicts with existing similarly named groups.  For readability, we prefix the real name.
	name := resource.NewUniqueHex(obj.Name+"-", maxSecurityGroupName, sha1.Size)
	fmt.Printf("Creating EC2 security group with name '%v'\n", name)
	create := &ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(name),
		Description: &obj.GroupDescription,
		VpcId:       obj.VPC.StringPtr(),
	}

	// Now go ahead and perform the action.
	result, err := p.ctx.EC2().CreateSecurityGroup(create)
	if err != nil {
		return "", nil, err
	}
	contract.Assert(result != nil)
	contract.Assert(result.GroupId != nil)
	out := &rpc.SecurityGroupOuts{GroupID: *result.GroupId}

	// For security groups in a default VPC, we return the ID; otherwise, the name.
	var id string
	if obj.VPC == nil {
		id = out.GroupID
	} else {
		id = name
	}
	fmt.Printf("EC2 security group created: %v; waiting for it to become active\n", id)

	// Don't proceed until the security group exists.
	if err = p.waitForSecurityGroupState(id, true); err != nil {
		return "", nil, err
	}

	// Authorize ingress rules if any exist.
	if ingress := obj.SecurityGroupIngress; ingress != nil && len(*ingress) > 0 {
		fmt.Printf("Authorizing %v security group ingress (inbound) rules\n", len(*ingress))
		for _, rule := range *ingress {
			if err := p.createSecurityGroupIngressRule(id, rule); err != nil {
				return "", nil, err
			}
		}
	}

	// Authorize egress rules if any exist.
	if egress := obj.SecurityGroupEgress; egress != nil && len(*egress) > 0 {
		fmt.Printf("Authorizing %v security group egress (outbound) rules\n", len(*egress))
		for _, rule := range *egress {
			if err := p.createSecurityGroupEgressRule(id, rule); err != nil {
				return "", nil, err
			}
		}
	}

	return id, out, nil
}

// Get reads the instance state identified by ID, returning a populated resource object, or an error if not found.
func (p *sgProvider) Get(ctx context.Context, id string) (*rpc.SecurityGroup, error) {
	return nil, errors.New("Not yet implemented")
}

// InspectChange checks what impacts a hypothetical update will have on the resource's properties.
func (p *sgProvider) InspectChange(ctx context.Context, id string,
	old *rpc.SecurityGroup, new *rpc.SecurityGroup, diff *resource.ObjectDiff) ([]string, error) {
	return nil, nil
}

// Update updates an existing resource with new values.  Only those values in the provided property bag are updated
// to new values.  The resource ID is returned and may be different if the resource had to be recreated.
func (p *sgProvider) Update(ctx context.Context, id string,
	old *rpc.SecurityGroup, new *rpc.SecurityGroup, diff *resource.ObjectDiff) error {
	// If only the ingress and/or egress rules changed, we can incrementally apply the updates.
	gresses := []struct {
		key    resource.PropertyKey
		olds   *[]rpc.SecurityGroupRule
		news   *[]rpc.SecurityGroupRule
		create func(string, rpc.SecurityGroupRule) error
		delete func(string, rpc.SecurityGroupRule) error
	}{
		{
			rpc.SecurityGroup_SecurityGroupIngress,
			new.SecurityGroupIngress,
			old.SecurityGroupIngress,
			p.createSecurityGroupIngressRule,
			p.deleteSecurityGroupIngressRule,
		},
		{
			rpc.SecurityGroup_SecurityGroupEgress,
			new.SecurityGroupEgress,
			old.SecurityGroupEgress,
			p.createSecurityGroupEgressRule,
			p.deleteSecurityGroupEgressRule,
		},
	}
	for _, gress := range gresses {
		if diff.Changed(gress.key) {
			// First accumulate the diffs.
			var creates []rpc.SecurityGroupRule
			var deletes []rpc.SecurityGroupRule
			if diff.Added(gress.key) {
				contract.Assert(gress.news != nil && len(*gress.news) > 0)
				for _, rule := range *gress.news {
					creates = append(creates, rule)
				}
			} else if diff.Deleted(gress.key) {
				contract.Assert(gress.olds != nil && len(*gress.olds) > 0)
				for _, rule := range *gress.olds {
					deletes = append(deletes, rule)
				}
			} else if diff.Updated(gress.key) {
				update := diff.Updates[gress.key]
				contract.Assert(update.Array != nil)
				for _, add := range update.Array.Adds {
					contract.Assert(add.IsObject())
					var rule rpc.SecurityGroupRule
					if err := mapper.MapIU(add.ObjectValue().Mappable(), &rule); err != nil {
						return err
					}
					creates = append(creates, rule)
				}
				for _, delete := range update.Array.Deletes {
					contract.Assert(delete.IsObject())
					var rule rpc.SecurityGroupRule
					if err := mapper.MapIU(delete.ObjectValue().Mappable(), &rule); err != nil {
						return err
					}
					deletes = append(deletes, rule)
				}
				for _, change := range update.Array.Updates {
					// We can't update individual fields of a rule; simply delete and recreate.
					var before rpc.SecurityGroupRule
					contract.Assert(change.Old.IsObject())
					if err := mapper.MapIU(change.Old.ObjectValue().Mappable(), &before); err != nil {
						return err
					}
					deletes = append(deletes, before)
					var after rpc.SecurityGroupRule
					contract.Assert(change.New.IsObject())
					if err := mapper.MapIU(change.New.ObjectValue().Mappable(), &after); err != nil {
						return err
					}
					creates = append(creates, after)
				}
			}

			// And now actually perform the create and delete operations.
			for _, delete := range deletes {
				if err := p.deleteSecurityGroupIngressRule(id, delete); err != nil {
					return err
				}
			}
			for _, create := range creates {
				if err := p.createSecurityGroupIngressRule(id, create); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
func (p *sgProvider) Delete(ctx context.Context, id string) error {
	// First, perform the deletion.
	fmt.Printf("Terminating EC2 SecurityGroup '%v'\n", id)
	delete := &ec2.DeleteSecurityGroupInput{GroupId: aws.String(id)}
	if _, err := p.ctx.EC2().DeleteSecurityGroup(delete); err != nil {
		return err
	}

	fmt.Printf("EC2 Security Group delete request submitted; waiting for it to terminate\n")

	// Don't finish the operation until the security group exists.
	return p.waitForSecurityGroupState(id, false)
}

func (p *sgProvider) crudSecurityGroupRule(prefix, kind string, rule rpc.SecurityGroupRule,
	action func(from *int64, to *int64) error) error {
	// First print a little status to stdout.
	fmt.Printf("%v security group %v rule: IPProtocol=%v", prefix, kind, rule.IPProtocol)
	if rule.CIDRIP != nil {
		fmt.Printf(", CIDRIP=%v", *rule.CIDRIP)
	}
	var from *int64
	if rule.FromPort != nil {
		fromPort := int64(*rule.FromPort)
		fmt.Printf(", FromPort=%v", fromPort)
		from = &fromPort
	}
	var to *int64
	if rule.ToPort != nil {
		toPort := int64(*rule.ToPort)
		fmt.Printf(", ToPort=%v", toPort)
		to = &toPort
	}
	fmt.Printf("\n")

	// Now perform the action and return its error (or nil) as our result.
	return action(from, to)
}

func (p *sgProvider) createSecurityGroupIngressRule(groupID string, rule rpc.SecurityGroupRule) error {
	return p.crudSecurityGroupRule("Authorizing", "ingress (inbound)", rule, func(from *int64, to *int64) error {
		_, err := p.ctx.EC2().AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
			GroupId:    aws.String(groupID),
			IpProtocol: aws.String(rule.IPProtocol),
			CidrIp:     rule.CIDRIP,
			FromPort:   from,
			ToPort:     to,
		})
		return err
	})
}

func (p *sgProvider) deleteSecurityGroupIngressRule(groupID string, rule rpc.SecurityGroupRule) error {
	return p.crudSecurityGroupRule("Revoking", "ingress (inbound)", rule, func(from *int64, to *int64) error {
		_, err := p.ctx.EC2().RevokeSecurityGroupIngress(&ec2.RevokeSecurityGroupIngressInput{
			GroupId:    aws.String(groupID),
			IpProtocol: aws.String(rule.IPProtocol),
			CidrIp:     rule.CIDRIP,
			FromPort:   from,
			ToPort:     to,
		})
		return err
	})
}

func (p *sgProvider) createSecurityGroupEgressRule(groupID string, rule rpc.SecurityGroupRule) error {
	return p.crudSecurityGroupRule("Authorizing", "egress (outbound)", rule, func(from *int64, to *int64) error {
		_, err := p.ctx.EC2().AuthorizeSecurityGroupEgress(&ec2.AuthorizeSecurityGroupEgressInput{
			GroupId:    aws.String(groupID),
			IpProtocol: aws.String(rule.IPProtocol),
			CidrIp:     rule.CIDRIP,
			FromPort:   from,
			ToPort:     to,
		})
		return err
	})
}

func (p *sgProvider) deleteSecurityGroupEgressRule(groupID string, rule rpc.SecurityGroupRule) error {
	return p.crudSecurityGroupRule("Revoking", "egress (outbound)", rule, func(from *int64, to *int64) error {
		_, err := p.ctx.EC2().RevokeSecurityGroupEgress(&ec2.RevokeSecurityGroupEgressInput{
			GroupId:    aws.String(groupID),
			IpProtocol: aws.String(rule.IPProtocol),
			CidrIp:     rule.CIDRIP,
			FromPort:   from,
			ToPort:     to,
		})
		return err
	})
}

func (p *sgProvider) waitForSecurityGroupState(id string, exist bool) error {
	succ, err := awsctx.RetryUntil(
		p.ctx,
		func() (bool, error) {
			req := &ec2.DescribeSecurityGroupsInput{GroupIds: []*string{aws.String(id)}}
			missing := true
			res, err := p.ctx.EC2().DescribeSecurityGroups(req)
			if err != nil {
				if !isSecurityGroupNotExistErr(err) {
					return false, err // quit and propagate the error
				}
			} else if res != nil && len(res.SecurityGroups) > 0 {
				contract.Assert(len(res.SecurityGroups) == 1)
				contract.Assert(*res.SecurityGroups[0].GroupId == id)
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
		return fmt.Errorf("EC2 security group '%v' did not %v", id, reason)
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
