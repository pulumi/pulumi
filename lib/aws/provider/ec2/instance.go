// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package ec2

import (
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	awsec2 "github.com/aws/aws-sdk-go/service/ec2"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/util/contract"
	"github.com/pulumi/lumi/sdk/go/pkg/lumirpc"
	"golang.org/x/net/context"

	"github.com/pulumi/lumi/lib/aws/provider/arn"
	"github.com/pulumi/lumi/lib/aws/provider/awsctx"
	"github.com/pulumi/lumi/lib/aws/rpc/ec2"
)

const InstanceToken = ec2.InstanceToken

// NewInstanceProvider creates a provider that handles EC2 instance operations.
func NewInstanceProvider(ctx *awsctx.Context) lumirpc.ResourceProviderServer {
	ops := &instanceProvider{ctx}
	return ec2.NewInstanceProvider(ops)
}

type instanceProvider struct {
	ctx *awsctx.Context
}

// Check validates that the given property bag is valid for a resource of the given type.
func (p *instanceProvider) Check(ctx context.Context, obj *ec2.Instance, property string) error {
	switch property {
	case ec2.Instance_ImageID:
		// Check that the AMI exists; this catches misspellings, AMI region mismatches, accessibility problems, etc.
		result, err := p.ctx.EC2().DescribeImages(&awsec2.DescribeImagesInput{
			ImageIds: []*string{aws.String(obj.ImageID)},
		})
		if err != nil {
			return err
		}
		if len(result.Images) == 0 {
			return fmt.Errorf("missing image: %v", obj.ImageID)
		}
	}
	return nil
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transacational").
func (p *instanceProvider) Create(ctx context.Context, obj *ec2.Instance) (resource.ID, error) {
	// Create the create instances request object.
	var secgrpIDs []*string
	if obj.SecurityGroups != nil {
		for _, secgrpARN := range *obj.SecurityGroups {
			secgrpID, err := arn.ParseResourceName(secgrpARN)
			if err != nil {
				return "", err
			}
			secgrpIDs = append(secgrpIDs, aws.String(secgrpID))
		}
	}
	var instanceType *string
	if obj.InstanceType != nil {
		its := string(*obj.InstanceType)
		instanceType = &its
	}
	var tagSpecifications []*awsec2.TagSpecification
	if obj.Tags != nil {
		var tags []*awsec2.Tag
		for _, tag := range *obj.Tags {
			tags = append(tags, &awsec2.Tag{
				Key:   aws.String(tag.Key),
				Value: aws.String(tag.Value),
			})
		}
		tagSpecifications = []*awsec2.TagSpecification{{
			ResourceType: aws.String("instance"),
			Tags:         tags,
		}}
	}
	create := &awsec2.RunInstancesInput{
		ImageId:           aws.String(obj.ImageID),
		InstanceType:      instanceType,
		SecurityGroupIds:  secgrpIDs,
		KeyName:           obj.KeyName,
		MinCount:          aws.Int64(int64(1)),
		MaxCount:          aws.Int64(int64(1)),
		TagSpecifications: tagSpecifications,
	}

	// Now go ahead and perform the action.
	fmt.Print("Creating new EC2 instance resource\n")
	result, err := p.ctx.EC2().RunInstances(create)
	if err != nil {
		return "", err
	} else if result == nil || len(result.Instances) == 0 {
		return "", errors.New("EC2 instance created, but AWS did not return an instance ID for it")
	}

	// Get the unique ID from the created instance.
	contract.Assert(len(result.Instances) == 1)
	contract.Assert(result.Instances[0] != nil)
	contract.Assert(result.Instances[0].InstanceId != nil)
	id := aws.StringValue(result.Instances[0].InstanceId)

	// Before returning that all is okay, wait for the instance to reach the running state.
	fmt.Printf("EC2 instance '%v' created; now waiting for it to become 'running'\n", id)
	// TODO[pulumi/lumi#219]: if this fails, but the creation succeeded, we will have an orphaned resource; report this
	//     differently than other "benign" errors.
	if err = p.ctx.EC2().WaitUntilInstanceRunning(
		&awsec2.DescribeInstancesInput{InstanceIds: []*string{aws.String(id)}}); err != nil {
		return "", err
	}

	return arn.NewEC2InstanceID(p.ctx.Region(), p.ctx.AccountID(), id), nil
}

// Query returns an (possibly empty) array of instances.
func (p *instanceProvider) Query(ctx context.Context) ([]*ec2.Instance, error) {
	resp, err := p.ctx.EC2().DescribeInstances()
	var instances []*ec2.Instance
	if err != nil {
		if awsctx.IsAWSError(err, "InvalidInstanceID.NotFound") {
			return nil, nil
		}
		return nil, err
	} else if resp == nil || len(resp.Reservations) == 0 {
		return nil, nil
	}

	for inst := range resp.Reservations {
		var secgrpIDs *[]resource.ID
		if len(inst.SecurityGroups) > 0 {
			var ids []resource.ID
			for _, group := range inst.SecurityGroups {
				ids = append(ids,
					arn.NewEC2SecurityGroupID(idarn.Region, idarn.AccountID, aws.StringValue(group.GroupId)))
			}
			secgrpIDs = &ids
		}

		var instanceTags *[]ec2.Tag
		if len(inst.Tags) > 0 {
			var tags []ec2.Tag
			for _, tag := range inst.Tags {
				tags = append(tags, ec2.Tag{
					Key:   aws.StringValue(tag.Key),
					Value: aws.StringValue(tag.Value),
				})
			}
			instanceTags = &tags
		}

		instanceType := ec2.InstanceType(aws.StringValue(inst.InstanceType))
		instances = append(instances, &ec2.Instance{
			ImageID:          aws.StringValue(inst.ImageId),
			InstanceType:     &instanceType,
			SecurityGroups:   secgrpIDs,
			KeyName:          inst.KeyName,
			AvailabilityZone: aws.StringValue(inst.Placement.AvailabilityZone),
			PrivateDNSName:   inst.PrivateDnsName,
			PublicDNSName:    inst.PublicDnsName,
			PrivateIP:        inst.PrivateIpAddress,
			PublicIP:         inst.PublicIpAddress,
			Tags:             instanceTags,
		})
	}

	return instances, nil
}

// Get reads the instance state identified by ID, returning a populated resource object, or an nil if not found.
func (p *instanceProvider) Get(ctx context.Context, id resource.ID) (*ec2.Instance, error) {
	idarn, err := arn.ARN(id).Parse()
	if err != nil {
		return nil, err
	}
	iid := idarn.ResourceName()
	resp, err := p.ctx.EC2().DescribeInstances(
		&awsec2.DescribeInstancesInput{InstanceIds: []*string{aws.String(iid)}})
	if err != nil {
		if awsctx.IsAWSError(err, "InvalidInstanceID.NotFound") {
			return nil, nil
		}
		return nil, err
	} else if resp == nil || len(resp.Reservations) == 0 {
		return nil, nil
	}

	// If we are here, we know that there is a reservation that matched; read its fields and populate the object.
	contract.Assert(len(resp.Reservations) == 1)
	contract.Assert(resp.Reservations[0] != nil)
	contract.Assert(resp.Reservations[0].Instances != nil)
	contract.Assert(len(resp.Reservations[0].Instances) == 1)
	inst := resp.Reservations[0].Instances[0]

	var secgrpIDs *[]resource.ID
	if len(inst.SecurityGroups) > 0 {
		var ids []resource.ID
		for _, group := range inst.SecurityGroups {
			ids = append(ids,
				arn.NewEC2SecurityGroupID(idarn.Region, idarn.AccountID, aws.StringValue(group.GroupId)))
		}
		secgrpIDs = &ids
	}

	var instanceTags *[]ec2.Tag
	if len(inst.Tags) > 0 {
		var tags []ec2.Tag
		for _, tag := range inst.Tags {
			tags = append(tags, ec2.Tag{
				Key:   aws.StringValue(tag.Key),
				Value: aws.StringValue(tag.Value),
			})
		}
		instanceTags = &tags
	}

	instanceType := ec2.InstanceType(aws.StringValue(inst.InstanceType))
	return &ec2.Instance{
		ImageID:          aws.StringValue(inst.ImageId),
		InstanceType:     &instanceType,
		SecurityGroups:   secgrpIDs,
		KeyName:          inst.KeyName,
		AvailabilityZone: aws.StringValue(inst.Placement.AvailabilityZone),
		PrivateDNSName:   inst.PrivateDnsName,
		PublicDNSName:    inst.PublicDnsName,
		PrivateIP:        inst.PrivateIpAddress,
		PublicIP:         inst.PublicIpAddress,
		Tags:             instanceTags,
	}, nil
}

// InspectChange checks what impacts a hypothetical update will have on the resource's properties.
func (p *instanceProvider) InspectChange(ctx context.Context, id resource.ID,
	old *ec2.Instance, new *ec2.Instance, diff *resource.ObjectDiff) ([]string, error) {
	// TODO[pulumi/lumi#187]: we should permit changes to security groups for non-EC2-classic VMs that are in VPCs.
	// TODO[pulumi/lumi#241]: we should permit changes to instance type for EBS-backed instances.
	return nil, nil
}

// Update updates an existing resource with new values.  Only those values in the provided property bag are updated
// to new values.  The resource ID is returned and may be different if the resource had to be recreated.
func (p *instanceProvider) Update(ctx context.Context, id resource.ID,
	old *ec2.Instance, new *ec2.Instance, diff *resource.ObjectDiff) error {
	iid, err := arn.ParseResourceName(id)
	if err != nil {
		return err
	}
	if diff.Changed(ec2.Instance_Tags) {
		newTagSet := newTagHashSet(new.Tags)
		oldTagSet := newTagHashSet(old.Tags)
		d := oldTagSet.Diff(newTagSet)
		var addOrUpdateTags []*awsec2.Tag
		for _, o := range d.AddOrUpdates() {
			option := o.(tagHash).item
			addOrUpdateTags = append(addOrUpdateTags, &awsec2.Tag{
				Key:   aws.String(option.Key),
				Value: aws.String(option.Value),
			})
		}
		if len(addOrUpdateTags) > 0 {
			if _, tagerr := p.ctx.EC2().CreateTags(&awsec2.CreateTagsInput{
				Resources: []*string{aws.String(iid)},
				Tags:      addOrUpdateTags,
			}); tagerr != nil {
				return tagerr
			}
		}
		var deleteTags []*awsec2.Tag
		for _, o := range d.Deletes() {
			option := o.(tagHash).item
			deleteTags = append(deleteTags, &awsec2.Tag{
				Key:   aws.String(option.Key),
				Value: nil,
			})
		}
		if len(deleteTags) > 0 {
			_, err = p.ctx.EC2().DeleteTags(&awsec2.DeleteTagsInput{
				Resources: []*string{aws.String(iid)},
				Tags:      deleteTags,
			})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
func (p *instanceProvider) Delete(ctx context.Context, id resource.ID) error {
	iid, err := arn.ParseResourceName(id)
	if err != nil {
		return err
	}
	delete := &awsec2.TerminateInstancesInput{InstanceIds: []*string{aws.String(iid)}}
	fmt.Printf("Terminating EC2 instance '%v'\n", id)
	if _, err := p.ctx.EC2().TerminateInstances(delete); err != nil {
		return err
	}

	fmt.Print("EC2 instance termination request submitted; waiting for it to terminate\n")
	return p.ctx.EC2().WaitUntilInstanceTerminated(
		&awsec2.DescribeInstancesInput{InstanceIds: []*string{aws.String(iid)}})
}

type tagHash struct {
	item ec2.Tag
}

var _ awsctx.Hashable = tagHash{}

func (option tagHash) HashKey() awsctx.Hash {
	return awsctx.Hash(option.item.Key)
}
func (option tagHash) HashValue() awsctx.Hash {
	return awsctx.Hash(option.item.Key + ":" + option.item.Value)
}
func newTagHashSet(options *[]ec2.Tag) *awsctx.HashSet {
	set := awsctx.NewHashSet()
	if options == nil {
		return set
	}
	for _, option := range *options {
		set.Add(tagHash{option})
	}
	return set
}
