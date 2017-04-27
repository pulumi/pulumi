// Copyright 2017 Pulumi, Inc. All rights reserved.

package ec2

import (
	"github.com/pulumi/coconut/pkg/resource/idl"
)

// SecurityGroupEgress adds an egress (outbound) rule to an Amazon VPC security group.  For more information, see
// http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ec2-security-group-egress.html.
type SecurityGroupEgress struct {
	idl.NamedResource
	// Start of port range for the TCP and UDP protocols, or an ICMP type number. If you specify `icmp` for the
	// `ipProtocol` property, you can specify `-1` as a wildcard (i.e., any ICMP type number).
	FromPort float64 `coco:"fromPort,replaces"`
	// The Amazon VPC security group to modify.
	Group *SecurityGroup `coco:"group,replaces"`
	// IP protocol name or number.
	IPProtocol string `coco:"ipProtocol,replaces"`
	// End of port range for the TCP and UDP protocols, or an ICMP code. If you specify `icmp` for the `ipProtocol`
	// property, you can specify `-1` as a wildcard (i.e., any ICMP code).
	ToPort float64 `coco:"toPort,replaces"`
	// An IPv4 CIDR range.
	CIDRIP *string `coco:"cidrIp,replaces,optional"`
	// An IPv6 CIDR range.
	CIDRIPv6 *string `coco:"cidrIpv6,replaces,optional"`
	// The AWS service prefix of an Amazon VPC endpoint.
	DestinationPrefixListId *string `coco:"destinationPrefixListId,replaces,optional"`
	// Specifies the group ID of the destination Amazon VPC security group.
	DestinationSecurityGroup *SecurityGroup `coco:"destinationSecurityGroup,replaces,optional"`
}
