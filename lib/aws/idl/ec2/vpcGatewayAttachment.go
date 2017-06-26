// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package ec2

import (
	"github.com/pulumi/lumi/pkg/resource/idl"
)

// VPCGatewayAttachment attaches a gateway to a VPC.  For more information, see
// http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ec2-vpc-gateway-attachment.html.
type VPCGatewayAttachment struct {
	idl.NamedResource
	// The VPC to associate with this gateway.
	VPC *VPC `lumi:"vpc,replaces"`
	// The Internet gateway to attach to the VPC.
	InternetGateway *InternetGateway `lumi:"internetGateway,replaces"`
}
