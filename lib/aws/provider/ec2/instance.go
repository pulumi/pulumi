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
func (p *instanceProvider) Create(ctx context.Context, obj *ec2.Instance) (resource.ID, *ec2.InstanceOuts, error) {
	// Create the create instances request object.
	var secgrpIDs []*string
	if obj.SecurityGroups != nil {
		for _, id := range *obj.SecurityGroups {
			secgrpIDs = append(secgrpIDs, id.StringPtr())
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
		return "", nil, err
	}
	contract.Assert(result != nil)
	contract.Assert(len(result.Instances) == 1)
	inst := result.Instances[0]
	contract.Assert(inst != nil)
	id := inst.InstanceId
	contract.Assert(inst.InstanceId != nil)

	// Before returning that all is okay, wait for the instance to reach the running state.
	fmt.Fprintf(os.Stdout, "EC2 instance '%v' created; now waiting for it to become 'running'\n", *id)
	// TODO: if this fails, but the creation succeeded, we will have an orphaned resource; report this differently.
	if err = p.ctx.EC2().WaitUntilInstanceRunning(
		&awsec2.DescribeInstancesInput{InstanceIds: []*string{id}}); err != nil {
		return "", nil, err
	}

	// Fetch the availability zone for the instance.
	status, err := p.ctx.EC2().DescribeInstanceStatus(
		&awsec2.DescribeInstanceStatusInput{InstanceIds: []*string{id}})
	if err != nil {
		return "", nil, err
	}
	contract.Assert(status != nil)
	contract.Assert(len(status.InstanceStatuses) == 1)

	// Manufacture the output properties structure.
	return resource.ID(*id), &ec2.InstanceOuts{
		AvailabilityZone: *status.InstanceStatuses[0].AvailabilityZone,
		PrivateDNSName:   inst.PrivateDnsName,
		PublicDNSName:    inst.PublicDnsName,
		PrivateIP:        inst.PrivateIpAddress,
		PublicIP:         inst.PublicIpAddress,
	}, nil
}

// Get reads the instance state identified by ID, returning a populated resource object, or an error if not found.
func (p *instanceProvider) Get(ctx context.Context, id resource.ID) (*ec2.Instance, error) {
	return nil, errors.New("Not yet implemented")
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
	delete := &awsec2.TerminateInstancesInput{InstanceIds: []*string{id.StringPtr()}}

	fmt.Fprintf(os.Stdout, "Terminating EC2 instance '%v'\n", id)
	if _, err := p.ctx.EC2().TerminateInstances(delete); err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "EC2 instance termination request submitted; waiting for it to terminate\n")
	return p.ctx.EC2().WaitUntilInstanceTerminated(
		&awsec2.DescribeInstancesInput{InstanceIds: []*string{id.StringPtr()}})
}
