// Copyright 2017 Pulumi, Inc. All rights reserved.

package ec2

import (
	"github.com/pulumi/coconut/pkg/resource/idl"
)

// VPC is a Virtual Private Cloud with a specified CIDR block.  For more information, see
// http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ec2-vpc.html.
type VPC struct {
	idl.NamedResource
	// The CIDR block you want the VPC to cover.  For example, "10.0.0.0/16".
	CIDRBlock string `coco:"cidrBlock,replaces"`
	// The allowed tenancy of instances launched into the VPC.  "default" indicates that instances can be launched with
	// any tenancy, while "dedicated" indicates that any instance launched into the VPC automatically has dedicated
	// tenancy, unless you launch it with the default tenancy.
	InstanceTenancy InstanceTenancy `coco:"instanceTenancy,optional,replaces"`
	// Specifies whether DNS resolution is supported for the VPC.  If true, the Amazon DNS server resolves DNS hostnames
	// for your instances to their corresponding IP addresses; otherwise, it does not.  By default, the value is true.
	EnableDNSSupport bool `coco:"enableDnsSupport,optional"`
	// Specifies whether the instances launched in the VPC get DNS hostnames.  If this attribute is true, instances in
	// the VPC get DNS hostnames; otherwise, they do not.  You can only set enableDnsHostnames to true if you also set
	// the enableDnsSupport property to true.  By default, the value is set to false.
	EnableDNSHostnames bool `coco:"enableDnsHostnames,optional"`
}

type InstanceTenancy string

const (
	// Your instance runs on shared hardware.
	DefaultTenancy InstanceTenancy = "default"
	// Your instance runs on single-tenant hardware.
	DedicatedTenancy InstanceTenancy = "dedicated"
	// Your instance runs on a Dedicated Host, which is an isolated server with configurations that you can control.
	HostTenancy InstanceTenancy = "host"
)
