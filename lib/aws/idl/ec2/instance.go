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

// Instance ia an EC2 VM instance.  For more information, see
// http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-ec2-instance.html.
type Instance struct {
	idl.NamedResource
	// Provides the unique ID of the Amazon Machine Image (AMI) that was assigned during registration.
	ImageID string `lumi:"imageId"`
	// The instance type, such as t2.micro. The default type is "m3.medium".
	InstanceType *InstanceType `lumi:"instanceType,optional"`
	// A list that contains the Amazon EC2 security groups to assign to the Amazon EC2 instance.
	SecurityGroups *[]*SecurityGroup `lumi:"securityGroups,optional"`
	// Provides the name of the Amazon EC2 key pair.
	KeyName *string `lumi:"keyName,optional"`

	// Output properties:

	// The Availability Zone where the specified instance is launched.  For example: `us-east-1b`.
	AvailabilityZone string `lumi:"availabilityZone,out"`
	// The private DNS name of the specified instance.  For example: `ip-10-24-34-0.ec2.internal`.
	PrivateDNSName *string `lumi:"privateDNSName,out,optional"`
	// The public DNS name of the specified instance.  For example: `ec2-107-20-50-45.compute-1.amazonaws.com`.
	PublicDNSName *string `lumi:"publicDNSName,out,optional"`
	// The private IP address of the specified instance.  For example: `10.24.34.0`.
	PrivateIP *string `lumi:"privateIP,out,optional"`
	// The public IP address of the specified instance.  For example: `192.0.2.0`.
	PublicIP *string `lumi:"publicIP,out,optional"`
}

// InstanceType is an enum type with all the names of instance types available in EC2.
type InstanceType string

const (
	// GENERAL PURPOSE:

	// T2: Instances are Burstable Performance Instances that provide a baseline level of CPU performance with the
	// ability to burst above the baseline. The baseline performance and ability to burst are governed by CPU Credits.
	// Each T2 instance receives CPU Credits continuously at a set rate depending on the instance size.  T2 instances
	// accrue CPU Credits when they are idle, and use CPU credits when they are active.  T2 instances are a good choice
	// for workloads that don’t use the full CPU often or consistently, but occasionally need to burst (e.g. web
	// servers, developer environments and databases). For more information see Burstable Performance Instances.
	T2InstanceNano    InstanceType = "t2.nano"
	T2InstanceMicro   InstanceType = "t2.micro"
	T2InstanceSmall   InstanceType = "t2.small"
	T2InstanceMedium  InstanceType = "t2.medium"
	T2InstanceLarge   InstanceType = "t2.large"
	T2InstanceXLarge  InstanceType = "t2.xlarge"
	T2Instance2XLarge InstanceType = "t2.2xlarge"

	// M4: Instances are the latest generation of General Purpose Instances. This family provides a balance of compute,
	// memory, and network resources, and it is a good choice for many applications.
	M4InstanceLarge    InstanceType = "m4.large"
	M4InstanceXLarge   InstanceType = "m4.xlarge"
	M4Instance2XLarge  InstanceType = "m4.2xlarge"
	M4Instance4XLarge  InstanceType = "m4.4xlarge"
	M4Instance10XLarge InstanceType = "m4.10xlarge"
	M4Instance16XLarge InstanceType = "m4.16xlarge"

	// M3: This family includes the M3 instance types and provides a balance of compute, memory, and network resources,
	// and it is a good choice for many applications.
	M3InstanceMedium  InstanceType = "m3.medium"
	M3InstanceLarge   InstanceType = "m3.large"
	M3InstanceXLarge  InstanceType = "m3.xlarge"
	M3Instance2XLarge InstanceType = "m3.2xlarge"

	// COMPUTE OPTIMIZED:

	// C4: Instances are the latest generation of Compute-optimized instances, featuring the highest performing
	// processors and the lowest price/compute performance in EC2.
	C4InstanceLarge   InstanceType = "c4.large"
	C4InstanceXLarge  InstanceType = "c4.xlarge"
	C4Instance2XLarge InstanceType = "c4.2xlarge"
	C4Instance4XLarge InstanceType = "c4.4xlarge"
	C4Instance8XLarge InstanceType = "c4.8xlarge"

	// C3: Instances are the previous generation of Compute-optimized instances.
	C3InstanceLarge   InstanceType = "c3.large"
	C3InstanceXLarge  InstanceType = "c3.xlarge"
	C3Instance2XLarge InstanceType = "c3.2xlarge"
	C3Instance4XLarge InstanceType = "c3.4xlarge"
	C3Instance8XLarge InstanceType = "c3.8xlarge"

	// MEMORY OPTIMIZED:

	// X1: Instances are optimized for large-scale, enterprise-class, in-memory applications and have the lowest price
	// per GiB of RAM among Amazon EC2 instance types.
	X1Instance32XLarge InstanceType = "x1.32xlarge"
	X1Instance16XLarge InstanceType = "x1.16xlarge"

	// R4: Instance InstanceType =s are optimized for memory-intensive applications and offer better price per GiB of RAM than R3.
	R4InstanceLarge    InstanceType = "r4.large"
	R4InstanceXLarge   InstanceType = "r4.xlarge"
	R4Instance2XLarge  InstanceType = "r4.2xlarge"
	R4Instance4XLarge  InstanceType = "r4.4xlarge"
	R4Instance8XLarge  InstanceType = "r4.8xlarge"
	R4Instance16XLarge InstanceType = "r4.16xlarge"

	// R3: Instance InstanceType =s are optimized for memory-intensive applications and offer lower price per GiB of RAM.
	R3InstanceLarge   InstanceType = "r3.large"
	R3InstanceXLarge  InstanceType = "r3.xlarge"
	R3Instance2XLarge InstanceType = "r3.2xlarge"
	R3Instance4XLarge InstanceType = "r3.4xlarge"
	R3Instance8XLarge InstanceType = "r3.8xlarge"

	// ACCELERATED COMPUTING INSTANCES:

	// P2: Instance InstanceType =s are intended for general-purpose GPU compute applications.
	P2InstanceXLarge   InstanceType = "p2.xlarge"
	P2Instance8XLarge  InstanceType = "p2.8xlarge"
	P2Instance16XLarge InstanceType = "p2.16xlarge"

	// G2: Instances are optimized for graphics-intensive applications.
	G2Instance2XLarge InstanceType = "g2.2xlarge"
	G2Instance8XLarge InstanceType = "g2.8xlarge"

	// F1: Instances offer customizable hardware acceleration with field programmable gate arrays (FPGAs).
	F1Instance2XLarge  InstanceType = "f1.2xlarge"
	F1Instance16XLarge InstanceType = "f1.16xlarge"

	// STORAGE OPTIMIZED:

	// I3: This family includes the High Storage Instances that provide Non-Volatile Memory Express (NVMe) SSD backed
	// instance storage optimized for low latency, very high random I/O performance, high sequential read throughput and
	// provide high IOPS at a low cost.
	I3InstanceLarge    InstanceType = "i3.large"
	I3InstanceXLarge   InstanceType = "i3.xlarge"
	I3Instance2XLarge  InstanceType = "i3.2xlarge"
	I3Instance4XLarge  InstanceType = "i3.4xlarge"
	I3Instance8XLarge  InstanceType = "i3.8xlarge"
	I3Instance16XLarge InstanceType = "i3.16xlarge"

	// D2: Instances feature up to 48 TB of HDD-based local storage, deliver high disk throughput, and offer the lowest
	// price per disk throughput performance on Amazon EC2.
	D2InstanceXLarge  InstanceType = "d2.xlarge"
	D2Instance2XLarge InstanceType = "d2.2xlarge"
	D2Instance4XLarge InstanceType = "d2.4xlarge"
	D2Instance8XLarge InstanceType = "d2.8xlarge"
)
