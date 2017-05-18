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
	"github.com/pulumi/lumi/pkg/resource/idl"
)

// A SecurityGroup is an Amazon EC2 Security Group.  For more information, see
// http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-ec2-security-group.html.
type SecurityGroup struct {
	idl.NamedResource
	// A required description about the security group.
	GroupDescription string `lumi:"groupDescription,replaces"`
	// The VPC in which this security group resides (or blank if the default VPC).
	VPC *VPC `lumi:"vpc,optional,replaces"`
	// A list of Amazon EC2 security group egress rules.
	SecurityGroupEgress *[]SecurityGroupRule `lumi:"securityGroupEgress,optional"`
	// A list of Amazon EC2 security group ingress rules.
	SecurityGroupIngress *[]SecurityGroupRule `lumi:"securityGroupIngress,optional"`
	// The group ID of the specified security group, such as `sg-94b3a1f6`.
	GroupID string `lumi:"groupID,out"`
}

// A SecurityGroupRule describes an EC2 security group rule embedded within a SecurityGroup.
type SecurityGroupRule struct {
	// The IP name or number.
	IPProtocol string `lumi:"ipProtocol"`
	// Specifies a CIDR range.
	CIDRIP *string `lumi:"cidrIp,optional"`
	// The start of port range for the TCP and UDP protocols, or an ICMP type number.  An ICMP type number of `-1`
	// indicates a wildcard (i.e., any ICMP type number).
	FromPort *float64 `lumi:"fromPort,optional"`
	// The end of port range for the TCP and UDP protocols, or an ICMP code.  An ICMP code of `-1` indicates a
	// wildcard (i.e., any ICMP code).
	ToPort *float64 `lumi:"toPort,optional"`
}
