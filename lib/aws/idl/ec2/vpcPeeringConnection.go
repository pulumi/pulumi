// Copyright 2017 Pulumi, Inc. All rights reserved.

package ec2

import (
	"github.com/pulumi/lumi/pkg/resource/idl"
)

// VPCPeeringConnection enables a network connection between two virtual private clouds (VPCs) so that you can route
// traffic between them by means of a private IP addresses.  For more information, see
// http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ec2-vpcpeeringconnection.html.
type VPCPeeringConnection struct {
	idl.NamedResource
	// The VPC with which you are creating the peering connection.
	PeerVPC *VPC `lumi:"peerVpc,replaces"`
	// The VPC that is requesting a peering connection.
	VPC *VPC `lumi:"vpc,replaces"`
}
