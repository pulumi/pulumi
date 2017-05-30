// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ec2

import (
	"errors"
	"fmt"
	"os"
	"reflect"

	"github.com/aws/aws-sdk-go/aws"
	awsec2 "github.com/aws/aws-sdk-go/service/ec2"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/util/contract"
	"github.com/pulumi/lumi/pkg/util/mapper"
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
func (p *instanceProvider) Check(ctx context.Context, obj *ec2.Instance) ([]mapper.FieldError, error) {
	var failures []mapper.FieldError
	if obj.ImageID != "" {
		// Check that the AMI exists; this catches misspellings, AMI region mismatches, accessibility problems, etc.
		result, err := p.ctx.EC2().DescribeImages(&awsec2.DescribeImagesInput{
			ImageIds: []*string{aws.String(obj.ImageID)},
		})
		if err != nil {
			return nil, err
		}
		if len(result.Images) == 0 {
			failures = append(failures,
				mapper.NewFieldErr(reflect.TypeOf(obj), ec2.Instance_ImageID,
					fmt.Errorf("missing image: %v", obj.ImageID)))
		} else {
			contract.Assertf(len(result.Images) == 1, "Did not expect multiple instance matches")
			contract.Assertf(result.Images[0].ImageId != nil, "Expected a non-nil matched instance ID")
			contract.Assertf(*result.Images[0].ImageId == obj.ImageID, "Expected instance IDs to match")
		}
	}
	return failures, nil
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
	create := &awsec2.RunInstancesInput{
		ImageId:          aws.String(obj.ImageID),
		InstanceType:     instanceType,
		SecurityGroupIds: secgrpIDs,
		KeyName:          obj.KeyName,
		MinCount:         aws.Int64(int64(1)),
		MaxCount:         aws.Int64(int64(1)),
	}

	// Now go ahead and perform the action.
	fmt.Fprintf(os.Stdout, "Creating new EC2 instance resource\n")
	result, err := p.ctx.EC2().RunInstances(create)
	if err != nil {
		return "", err
	}

	// Get the unique ID from the created instance.
	contract.Assert(result != nil)
	contract.Assert(len(result.Instances) == 1)
	inst := result.Instances[0]
	contract.Assert(inst != nil)
	contract.Assert(inst.InstanceId != nil)
	id := *inst.InstanceId

	// Before returning that all is okay, wait for the instance to reach the running state.
	fmt.Fprintf(os.Stdout, "EC2 instance '%v' created; now waiting for it to become 'running'\n", id)
	// TODO: if this fails, but the creation succeeded, we will have an orphaned resource; report this differently.
	if err = p.ctx.EC2().WaitUntilInstanceRunning(
		&awsec2.DescribeInstancesInput{InstanceIds: []*string{aws.String(id)}}); err != nil {
		return "", err
	}

	return arn.NewEC2InstanceID(p.ctx.Region(), p.ctx.AccountID(), id), nil
}

// Get reads the instance state identified by ID, returning a populated resource object, or an nil if not found.
func (p *instanceProvider) Get(ctx context.Context, id resource.ID) (*ec2.Instance, error) {
	iid, err := arn.ParseResourceName(id)
	if err != nil {
		return nil, err
	}
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
	resv := resp.Reservations[0]
	contract.Assert(len(resp.Reservations[0].Instances) == 1)
	inst := resv.Instances[0]

	var secgrpIDs *[]resource.ID
	if len(inst.SecurityGroups) > 0 {
		var ids []resource.ID
		for _, group := range inst.SecurityGroups {
			// TODO: security groups in a custom VPC should get the GroupName, not the GroupId.
			ids = append(ids, resource.ID(*group.GroupId))
		}
		secgrpIDs = &ids
	}

	instanceType := ec2.InstanceType(*inst.InstanceType)
	return &ec2.Instance{
		ImageID:          *inst.ImageId,
		InstanceType:     &instanceType,
		SecurityGroups:   secgrpIDs,
		KeyName:          inst.KeyName,
		AvailabilityZone: *inst.Placement.AvailabilityZone,
		PrivateDNSName:   inst.PrivateDnsName,
		PublicDNSName:    inst.PublicDnsName,
		PrivateIP:        inst.PrivateIpAddress,
		PublicIP:         inst.PublicIpAddress,
	}, nil
}

// InspectChange checks what impacts a hypothetical update will have on the resource's properties.
func (p *instanceProvider) InspectChange(ctx context.Context, id resource.ID,
	old *ec2.Instance, new *ec2.Instance, diff *resource.ObjectDiff) ([]string, error) {
	// TODO: we should permit changes to security groups for non-EC2-classic VMs that are in VPCs.
	return nil, nil
}

// Update updates an existing resource with new values.  Only those values in the provided property bag are updated
// to new values.  The resource ID is returned and may be different if the resource had to be recreated.
func (p *instanceProvider) Update(ctx context.Context, id resource.ID,
	old *ec2.Instance, new *ec2.Instance, diff *resource.ObjectDiff) error {
	return errors.New("No known updatable instance properties")
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
func (p *instanceProvider) Delete(ctx context.Context, id resource.ID) error {
	iid, err := arn.ParseResourceName(id)
	if err != nil {
		return err
	}
	delete := &awsec2.TerminateInstancesInput{InstanceIds: []*string{aws.String(iid)}}
	fmt.Fprintf(os.Stdout, "Terminating EC2 instance '%v'\n", id)
	if _, err := p.ctx.EC2().TerminateInstances(delete); err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "EC2 instance termination request submitted; waiting for it to terminate\n")
	return p.ctx.EC2().WaitUntilInstanceTerminated(
		&awsec2.DescribeInstancesInput{InstanceIds: []*string{aws.String(iid)}})
}
