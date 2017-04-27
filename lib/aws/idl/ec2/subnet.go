// Copyright 2017 Pulumi, Inc. All rights reserved.

package ec2

import (
	"github.com/pulumi/coconut/pkg/resource/idl"
)

// Subnet is a subnet in an existing VPC.  For more information, see
// http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ec2-subnet.html.
type Subnet struct {
	idl.NamedResource
	// The CIDR block that you want the subnet to cover (for example, `"10.0.0.0/24"`).
	CIDRBlock string `coco:"cidrBlock,replaces"`
	// The VPC on which you want to create the subnet.
	VPC *VPC `coco:"vpc,replaces"`
	// The availability zone in which you want the subnet.  By default, AWS selects a zone for you.
	AvailabilityZone *string `coco:"availabilityZone,replaces,optional"`
	// Indicates whether instances that are launched in this subnet receive a public IP address.  By default, `false`.
	MapPublicIpOnLaunch *bool `coco:"mapPublicIpOnLaunch,optional"`
}
