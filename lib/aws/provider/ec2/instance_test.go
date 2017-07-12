// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package ec2

import (
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	awsec2 "github.com/aws/aws-sdk-go/service/ec2"
	"github.com/pulumi/lumi/lib/aws/provider/awsctx"
	"github.com/pulumi/lumi/lib/aws/provider/testutil"
	"github.com/pulumi/lumi/lib/aws/rpc/ec2"
	"github.com/pulumi/lumi/pkg/resource"
)

var amis = map[string]string{
	"us-east-1":      "ami-6869aa05",
	"us-west-2":      "ami-7172b611",
	"us-west-1":      "ami-31490d51",
	"eu-west-1":      "ami-f9dd458a",
	"eu-west-2":      "ami-886369ec",
	"eu-central-1":   "ami-ea26ce85",
	"ap-northeast-1": "ami-374db956",
	"ap-northeast-2": "ami-2b408b45",
	"ap-southeast-1": "ami-a59b49c6",
	"ap-southeast-2": "ami-dc361ebf",
	"ap-south-1":     "ami-ffbdd790",
	"us-east-2":      "ami-f6035893",
	"ca-central-1":   "ami-730ebd17",
	"sa-east-1":      "ami-6dd04501",
	"cn-north-1":     "ami-8e6aa0e3",
}

func Test(t *testing.T) {
	t.Parallel()

	prefix := resource.NewUniqueHex("lumitest", 20, 20)
	ctx := testutil.CreateContext(t)
	defer cleanup(prefix, ctx)
	instanceType := ec2.InstanceType("t2.nano")

	testutil.ProviderTestSimple(t, NewInstanceProvider(ctx), InstanceToken, []interface{}{
		&ec2.Instance{
			Name:         aws.String(prefix),
			InstanceType: &instanceType,
			ImageID:      amis[ctx.Region()],
			Tags: &[]ec2.Tag{{
				Key:   prefix,
				Value: prefix,
			}},
		},
		&ec2.Instance{
			Name:         aws.String(prefix),
			InstanceType: &instanceType,
			ImageID:      amis[ctx.Region()],
			Tags: &[]ec2.Tag{{
				Key:   prefix,
				Value: prefix,
			}, {
				Key:   "Hello",
				Value: "World",
			}},
		},
	})

}

func cleanup(prefix string, ctx *awsctx.Context) {
	fmt.Printf("Cleaning up instances with tag:%v=%v\n", prefix, prefix)
	list, err := ctx.EC2().DescribeInstances(&awsec2.DescribeInstancesInput{
		Filters: []*awsec2.Filter{{
			Name:   aws.String("tag:" + prefix),
			Values: []*string{aws.String(prefix)},
		}},
	})
	if err != nil {
		return
	}
	cleaned := 0
	instanceIds := []*string{}
	for _, reservation := range list.Reservations {
		for _, instance := range reservation.Instances {
			if aws.StringValue(instance.State.Name) != awsec2.InstanceStateNameTerminated {
				instanceIds = append(instanceIds, instance.InstanceId)
				cleaned++
			}
		}
	}
	if len(instanceIds) > 0 {
		_, err = ctx.EC2().TerminateInstances(&awsec2.TerminateInstancesInput{
			InstanceIds: instanceIds,
		})
		if err != nil {
			fmt.Printf("Failed cleaning up %v tables: %v\n", cleaned, err)
		} else {
			fmt.Printf("Cleaned up %v tables\n", cleaned)
		}
	}
}
