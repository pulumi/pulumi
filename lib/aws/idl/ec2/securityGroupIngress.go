// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package ec2

import (
	"github.com/pulumi/lumi/pkg/resource/idl"
)

// SecurityGroupIngress dds an ingress (inbound) rule to an Amazon VPC security group.  For more information, see
// http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ec2-security-group-ingress.html.
type SecurityGroupIngress struct {
	idl.NamedResource
	// IP protocol name or number.
	IPProtocol string `lumi:"ipProtocol,replaces"`
	// An IPv4 CIDR range.
	CIDRIP *string `lumi:"cidrIp,replaces,optional"`
	// An IPv6 CIDR range.
	CIDRIPv6 *string `lumi:"cidrIpv6,replaces,optional"`
	// Start of port range for the TCP and UDP protocols, or an ICMP type number. If you specify `icmp` for the
	// `ipProtocol` property, you can specify `-1` as a wildcard (i.e., any ICMP type number).
	FromPort *float64 `lumi:"fromPort,replaces,optional"`
	// The Amazon VPC security group to modify.
	Group *SecurityGroup `lumi:"group,replaces,optional"`
	// Name of the Amazon EC2 security group (non-VPC security group) to modify.
	GroupName *string `lumi:"groupName,replaces,optional"`
	// Specifies the ID of the source security group or uses the Ref intrinsic function to refer to the logical ID of a
	// security group defined in the same template.
	SourceSecurityGroup *SecurityGroup `lumi:"sourceSecurityGroup,replaces,optional"`
	// Specifies the name of the Amazon EC2 security group (non-VPC security group) to allow access or uses the Ref
	// intrinsic function to refer to the logical name of a security group defined in the same template. For instances
	// in a VPC, specify the SourceSecurityGroupId property.
	SourceSecurityGroupName *string `lumi:"sourceSecurityGroupName,replaces,optional"`
	// Specifies the AWS Account ID of the owner of the Amazon EC2 security group specified in the
	// SourceSecurityGroupName property.
	SourceSecurityGroupOwnerId *string `lumi:"sourceSecurityGroupOwnerId,replaces,optional"`
	// End of port range for the TCP and UDP protocols, or an ICMP code. If you specify `icmp` for the `ipProtocol`
	// property, you can specify `-1` as a wildcard (i.e., any ICMP code).
	ToPort *float64 `lumi:"toPort,replaces,optional"`
}
