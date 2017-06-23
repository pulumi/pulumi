// Copyright 2016-2017, Pulumi Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
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
	"github.com/pulumi/lumi/pkg/resource/idl"
)

// SecurityGroupEgress adds an egress (outbound) rule to an Amazon VPC security group.  For more information, see
// http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ec2-security-group-egress.html.
type SecurityGroupEgress struct {
	idl.NamedResource
	// Start of port range for the TCP and UDP protocols, or an ICMP type number. If you specify `icmp` for the
	// `ipProtocol` property, you can specify `-1` as a wildcard (i.e., any ICMP type number).
	FromPort float64 `lumi:"fromPort,replaces"`
	// The Amazon VPC security group to modify.
	Group *SecurityGroup `lumi:"group,replaces"`
	// IP protocol name or number.
	IPProtocol string `lumi:"ipProtocol,replaces"`
	// End of port range for the TCP and UDP protocols, or an ICMP code. If you specify `icmp` for the `ipProtocol`
	// property, you can specify `-1` as a wildcard (i.e., any ICMP code).
	ToPort float64 `lumi:"toPort,replaces"`
	// An IPv4 CIDR range.
	CIDRIP *string `lumi:"cidrIp,replaces,optional"`
	// An IPv6 CIDR range.
	CIDRIPv6 *string `lumi:"cidrIpv6,replaces,optional"`
	// The AWS service prefix of an Amazon VPC endpoint.
	DestinationPrefixListId *string `lumi:"destinationPrefixListId,replaces,optional"`
	// Specifies the group ID of the destination Amazon VPC security group.
	DestinationSecurityGroup *SecurityGroup `lumi:"destinationSecurityGroup,replaces,optional"`
}
