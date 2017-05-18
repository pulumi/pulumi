// Copyright 2017 Pulumi, Inc. All rights reserved.

package ec2

import (
	"github.com/pulumi/lumi/pkg/resource/idl"
)

// Route in a route table within a VPC.  For more information, see
// https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ec2-route.html.
type Route struct {
	idl.NamedResource
	// The CIDR address block used for the destination match.  For example, `0.0.0.0/0`.  Routing decisions are based
	// on the most specific match.
	DestinationCidrBlock string `lumi:"destinationCidrBlock,replaces"`
	// The route table where the route will be added.
	RouteTable *RouteTable `lumi:"routeTable,replaces"`
	// The Internet gateway that is attached to your VPC.  For route entries that specify a gateway, you must also
	// specify a dependency on the gateway attachment resource (`vpcGatewayAttachment`).
	InternetGateway *InternetGateway `lumi:"internetGateway,replaces"`
	// The gateway attachment resource that attached the specified gateway to the VPC.
	VPCGatewayAttachment *VPCGatewayAttachment `lumi:"vpcGatewayAttachment,replaces"`
}
