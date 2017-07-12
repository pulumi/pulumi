// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package ec2

import (
	"crypto/sha1"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	awsec2 "github.com/aws/aws-sdk-go/service/ec2"
	"github.com/pkg/errors"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/util/contract"
	"github.com/pulumi/lumi/pkg/util/convutil"
	"github.com/pulumi/lumi/pkg/util/mapper"
	"github.com/pulumi/lumi/sdk/go/pkg/lumirpc"
	"golang.org/x/net/context"

	"github.com/pulumi/lumi/lib/aws/provider/arn"
	"github.com/pulumi/lumi/lib/aws/provider/awsctx"
	"github.com/pulumi/lumi/lib/aws/rpc/ec2"
)

const SecurityGroupToken = ec2.SecurityGroupToken

// constants for the various security group limits.
const (
	maxSecurityGroupName        = 255
	maxSecurityGroupDescription = 255
)

// NewSecurityGroupProvider creates a provider that handles EC2 security group operations.
func NewSecurityGroupProvider(ctx *awsctx.Context) lumirpc.ResourceProviderServer {
	ops := &sgProvider{ctx}
	return ec2.NewSecurityGroupProvider(ops)
}

type sgProvider struct {
	ctx *awsctx.Context
}

// Check validates that the given property bag is valid for a resource of the given type.
func (p *sgProvider) Check(ctx context.Context, obj *ec2.SecurityGroup, property string) error {
	switch property {
	case ec2.SecurityGroup_GroupDescription:
		if len(obj.GroupDescription) > maxSecurityGroupDescription {
			return fmt.Errorf("exceeded maximum length of %v", maxSecurityGroupDescription)
		}
	case ec2.SecurityGroup_SecurityGroupEgress:
		if obj.VPC == nil && obj.SecurityGroupEgress != nil && len(*obj.SecurityGroupEgress) > 0 {
			return fmt.Errorf("custom egress rules are not supported on EC2-Classic groups (those without a VPC)")
		}
	}
	return nil
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transacational").
func (p *sgProvider) Create(ctx context.Context, obj *ec2.SecurityGroup) (resource.ID, error) {
	// Make the security group creation parameters.  If the developer specified a name, we will honor it, although we
	// prefer to auto-generate it from the Lumi resource name, suffixed with a hash, to avoid collisions.
	var name string
	if obj.GroupName == nil {
		name = resource.NewUniqueHex(*obj.Name+"-", maxSecurityGroupName, sha1.Size)
	} else {
		name = *obj.GroupName
	}
	var vpcID *string
	if obj.VPC != nil {
		vpc, err := arn.ParseResourceName(*obj.VPC)
		if err != nil {
			return "", err
		}
		vpcID = &vpc
	}

	fmt.Printf("Creating EC2 security group with name '%v'\n", name)
	create := &awsec2.CreateSecurityGroupInput{
		GroupName:   aws.String(name),
		Description: &obj.GroupDescription,
		VpcId:       vpcID,
	}

	// Now go ahead and perform the action.
	result, err := p.ctx.EC2().CreateSecurityGroup(create)
	if err != nil {
		return "", err
	} else if result == nil || result.GroupId == nil {
		return "", errors.New("EC2 security group created, but AWS did not return an ID for it")
	}

	id := aws.StringValue(result.GroupId)

	// Don't proceed until the security group exists.
	fmt.Printf("EC2 security group created: %v; waiting for it to become active\n", id)
	if err = p.waitForSecurityGroupState(id, true); err != nil {
		return "", err
	}

	// Authorize ingress rules if any exist.
	if ingress := obj.SecurityGroupIngress; ingress != nil && len(*ingress) > 0 {
		fmt.Printf("Authorizing %v security group ingress (inbound) rules\n", len(*ingress))
		for _, rule := range *ingress {
			if err := p.createSecurityGroupIngressRule(id, rule); err != nil {
				return "", err
			}
		}
	}

	// Authorize egress rules if any exist.
	if egress := obj.SecurityGroupEgress; egress != nil && len(*egress) > 0 {
		fmt.Printf("Authorizing %v security group egress (outbound) rules\n", len(*egress))
		for _, rule := range *egress {
			if err := p.createSecurityGroupEgressRule(id, rule); err != nil {
				return "", err
			}
		}
	}

	return arn.NewEC2SecurityGroupID(p.ctx.Region(), p.ctx.AccountID(), id), nil
}

func createSecurityGroupRulesFromIPPermissions(perms []*awsec2.IpPermission) *[]ec2.SecurityGroupRule {
	var ret *[]ec2.SecurityGroupRule
	if len(perms) > 0 {
		var rules []ec2.SecurityGroupRule
		for _, perm := range perms {
			rule := ec2.SecurityGroupRule{
				IPProtocol: *perm.IpProtocol,
				FromPort:   convutil.Int64PToFloat64P(perm.FromPort),
				ToPort:     convutil.Int64PToFloat64P(perm.FromPort),
			}

			// Although each unique entry is authorized individually, describe groups them together.  We must ungroup
			// them here in order for the output and input sets to match (i.e., one entry per IP address).
			contract.Assertf(len(perm.Ipv6Ranges) == 0, "IPv6 ranges not yet supported")
			if len(perm.IpRanges) > 0 {
				for _, rang := range perm.IpRanges {
					rule.CIDRIP = rang.CidrIp
					rules = append(rules, rule)
				}
			} else {
				rules = append(rules, rule)
			}
		}
		ret = &rules
	}
	return ret
}

// Query returns an (possibly empty) array of resource objects.
func (p *sgProvider) Query(ctx context.Context) ([]*ec2.SecurityGroup, error) {
	resp, err := p.ctx.EC2().DescribeSecurityGroups(&awsec2.DescribeSecurityGroupsInput{})
	if err != nil {
		return nil, err
	} else if resp == nil || len(resp.SecurityGroups) == 0 {
		return nil, nil
	}

	var grps []*ec2.SecurityGroup
	for _, grp := range resp.SecurityGroups {
		var vpcID *resource.ID
		if grp.VpcId != nil {
			vpc := arn.NewEC2VPCID(p.ctx.Region(), p.ctx.AccountID(), aws.StringValue(grp.VpcId))
			vpcID = &vpc
		}

		grps = append(grps, &ec2.SecurityGroup{
			GroupID:              aws.StringValue(grp.GroupId),
			GroupName:            grp.GroupName,
			GroupDescription:     aws.StringValue(grp.Description),
			VPC:                  vpcID,
			SecurityGroupEgress:  createSecurityGroupRulesFromIPPermissions(grp.IpPermissionsEgress),
			SecurityGroupIngress: createSecurityGroupRulesFromIPPermissions(grp.IpPermissions),
		})
	}

	return grps, nil
}

// Get reads the instance state identified by ID, returning a populated resource object, or an error if not found.
func (p *sgProvider) Get(ctx context.Context, id resource.ID) (*ec2.SecurityGroup, error) {
	gid, err := arn.ParseResourceName(id)
	if err != nil {
		return nil, err
	}
	resp, err := p.ctx.EC2().DescribeSecurityGroups(&awsec2.DescribeSecurityGroupsInput{
		GroupIds: []*string{aws.String(gid)},
	})
	if err != nil {
		if awsctx.IsAWSError(err, "InvalidSecurityGroupID.NotFound") {
			return nil, nil
		}
		return nil, err
	} else if resp == nil || len(resp.SecurityGroups) == 0 {
		return nil, nil
	}

	// If we found one, fetch all the requisite properties and store them on the output.
	grp := resp.SecurityGroups[0]

	var vpcID *resource.ID
	if grp.VpcId != nil {
		vpc := arn.NewEC2VPCID(p.ctx.Region(), p.ctx.AccountID(), aws.StringValue(grp.VpcId))
		vpcID = &vpc
	}

	return &ec2.SecurityGroup{
		GroupID:              aws.StringValue(grp.GroupId),
		GroupName:            grp.GroupName,
		GroupDescription:     aws.StringValue(grp.Description),
		VPC:                  vpcID,
		SecurityGroupEgress:  createSecurityGroupRulesFromIPPermissions(grp.IpPermissionsEgress),
		SecurityGroupIngress: createSecurityGroupRulesFromIPPermissions(grp.IpPermissions),
	}, nil
}

// InspectChange checks what impacts a hypothetical update will have on the resource's properties.
func (p *sgProvider) InspectChange(ctx context.Context, id resource.ID,
	old *ec2.SecurityGroup, new *ec2.SecurityGroup, diff *resource.ObjectDiff) ([]string, error) {
	return nil, nil
}

// Update updates an existing resource with new values.  Only those values in the provided property bag are updated
// to new values.  The resource ID is returned and may be different if the resource had to be recreated.
func (p *sgProvider) Update(ctx context.Context, id resource.ID,
	old *ec2.SecurityGroup, new *ec2.SecurityGroup, diff *resource.ObjectDiff) error {
	gid, err := arn.ParseResourceName(id)
	if err != nil {
		return err
	}

	// If only the ingress and/or egress rules changed, we can incrementally apply the updates.
	gresses := []struct {
		key    resource.PropertyKey
		olds   *[]ec2.SecurityGroupRule
		news   *[]ec2.SecurityGroupRule
		create func(string, ec2.SecurityGroupRule) error
		delete func(string, ec2.SecurityGroupRule) error
	}{
		{
			ec2.SecurityGroup_SecurityGroupIngress,
			new.SecurityGroupIngress,
			old.SecurityGroupIngress,
			p.createSecurityGroupIngressRule,
			p.deleteSecurityGroupIngressRule,
		},
		{
			ec2.SecurityGroup_SecurityGroupEgress,
			new.SecurityGroupEgress,
			old.SecurityGroupEgress,
			p.createSecurityGroupEgressRule,
			p.deleteSecurityGroupEgressRule,
		},
	}
	for _, gress := range gresses {
		if diff.Changed(gress.key) {
			// First accumulate the diffs.
			var creates []ec2.SecurityGroupRule
			var deletes []ec2.SecurityGroupRule
			if diff.Added(gress.key) {
				contract.Assert(gress.news != nil && len(*gress.news) > 0)
				creates = append(creates, *gress.news...)
			} else if diff.Deleted(gress.key) {
				contract.Assert(gress.olds != nil && len(*gress.olds) > 0)
				deletes = append(deletes, *gress.olds...)
			} else if diff.Updated(gress.key) {
				update := diff.Updates[gress.key]
				contract.Assert(update.Array != nil)
				for _, add := range update.Array.Adds {
					contract.Assert(add.IsObject())
					var rule ec2.SecurityGroupRule
					if err := mapper.MapIU(add.ObjectValue().Mappable(), &rule); err != nil {
						return err
					}
					creates = append(creates, rule)
				}
				for _, delete := range update.Array.Deletes {
					contract.Assert(delete.IsObject())
					var rule ec2.SecurityGroupRule
					if err := mapper.MapIU(delete.ObjectValue().Mappable(), &rule); err != nil {
						return err
					}
					deletes = append(deletes, rule)
				}
				for _, change := range update.Array.Updates {
					// We can't update individual fields of a rule; simply delete and recreate.
					var before ec2.SecurityGroupRule
					contract.Assert(change.Old.IsObject())
					if err := mapper.MapIU(change.Old.ObjectValue().Mappable(), &before); err != nil {
						return err
					}
					deletes = append(deletes, before)
					var after ec2.SecurityGroupRule
					contract.Assert(change.New.IsObject())
					if err := mapper.MapIU(change.New.ObjectValue().Mappable(), &after); err != nil {
						return err
					}
					creates = append(creates, after)
				}
			}

			// And now actually perform the create and delete operations.
			for _, delete := range deletes {
				if err := p.deleteSecurityGroupIngressRule(gid, delete); err != nil {
					return err
				}
			}
			for _, create := range creates {
				if err := p.createSecurityGroupIngressRule(gid, create); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
func (p *sgProvider) Delete(ctx context.Context, id resource.ID) error {
	gid, err := arn.ParseResourceName(id)
	if err != nil {
		return err
	}

	// First, perform the deletion.
	fmt.Printf("Terminating EC2 SecurityGroup '%v'\n", id)
	delete := &awsec2.DeleteSecurityGroupInput{GroupId: aws.String(gid)}
	if _, err := p.ctx.EC2().DeleteSecurityGroup(delete); err != nil {
		return err
	}

	fmt.Printf("EC2 Security Group delete request submitted; waiting for it to terminate\n")

	// Don't finish the operation until the security group exists.
	return p.waitForSecurityGroupState(gid, false)
}

func (p *sgProvider) crudSecurityGroupRule(prefix, kind string, rule ec2.SecurityGroupRule,
	action func(from *int64, to *int64) error) error {
	// First print a little status to stdout.
	fmt.Printf("%v security group %v rule: IPProtocol=%v", prefix, kind, rule.IPProtocol)
	if rule.CIDRIP != nil {
		fmt.Printf(", CIDRIP=%v", *rule.CIDRIP)
	}
	fromPort := convutil.Float64PToInt64P(rule.FromPort)
	if fromPort != nil {
		fmt.Printf(", FromPort=%v", *fromPort)
	}
	toPort := convutil.Float64PToInt64P(rule.ToPort)
	if toPort != nil {
		fmt.Printf(", ToPort=%v", *toPort)
	}
	fmt.Printf("\n")

	// Now perform the action and return its error (or nil) as our result.
	return action(fromPort, toPort)
}

func (p *sgProvider) createSecurityGroupIngressRule(groupID string, rule ec2.SecurityGroupRule) error {
	return p.crudSecurityGroupRule("Authorizing", "ingress (inbound)", rule, func(from *int64, to *int64) error {
		_, err := p.ctx.EC2().AuthorizeSecurityGroupIngress(&awsec2.AuthorizeSecurityGroupIngressInput{
			GroupId:    aws.String(groupID),
			IpProtocol: aws.String(rule.IPProtocol),
			CidrIp:     rule.CIDRIP,
			FromPort:   from,
			ToPort:     to,
		})
		return err
	})
}

func (p *sgProvider) deleteSecurityGroupIngressRule(groupID string, rule ec2.SecurityGroupRule) error {
	return p.crudSecurityGroupRule("Revoking", "ingress (inbound)", rule, func(from *int64, to *int64) error {
		_, err := p.ctx.EC2().RevokeSecurityGroupIngress(&awsec2.RevokeSecurityGroupIngressInput{
			GroupId:    aws.String(groupID),
			IpProtocol: aws.String(rule.IPProtocol),
			CidrIp:     rule.CIDRIP,
			FromPort:   from,
			ToPort:     to,
		})
		return err
	})
}

func (p *sgProvider) createSecurityGroupEgressRule(groupID string, rule ec2.SecurityGroupRule) error {
	return p.crudSecurityGroupRule("Authorizing", "egress (outbound)", rule, func(from *int64, to *int64) error {
		_, err := p.ctx.EC2().AuthorizeSecurityGroupEgress(&awsec2.AuthorizeSecurityGroupEgressInput{
			GroupId:    aws.String(groupID),
			IpProtocol: aws.String(rule.IPProtocol),
			CidrIp:     rule.CIDRIP,
			FromPort:   from,
			ToPort:     to,
		})
		return err
	})
}

func (p *sgProvider) deleteSecurityGroupEgressRule(groupID string, rule ec2.SecurityGroupRule) error {
	return p.crudSecurityGroupRule("Revoking", "egress (outbound)", rule, func(from *int64, to *int64) error {
		_, err := p.ctx.EC2().RevokeSecurityGroupEgress(&awsec2.RevokeSecurityGroupEgressInput{
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
			req := &awsec2.DescribeSecurityGroupsInput{GroupIds: []*string{aws.String(id)}}
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
			}

			// If not missing and exist==true, we're good; else, keep retrying.
			return exist, nil
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
	return awsctx.IsAWSError(err,
		// The specified security group does not eixst; this error can occur because the ID of a recently created
		// security group has not propagated through the system.
		"InvalidGroup.NotFound",
		// The specified security group does not exist; if you are creating a network interface, ensure that
		// you specify a VPC security group, and not an EC2-Classic security group.
		"InvalidSecurityGroupID.NotFound",
	)
}
