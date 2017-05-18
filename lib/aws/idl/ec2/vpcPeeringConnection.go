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
