// Copyright 2017 Pulumi, Inc. All rights reserved.

import {SecurityGroup} from './securityGroup';
import * as cloudformation from '../cloudformation';

// An EC2 instance.
// @name: aws/ec2/instance
// @website: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-ec2-instance.html
export class Instance
        extends cloudformation.Resource
        implements InstanceProperties {

    public imageId: string;
    public instanceType?: InstanceType;
    public securityGroups?: SecurityGroup[];
    public keyName?: string;

    constructor(name: string, args: InstanceProperties) {
        super({
            name: name,
            resource: "AWS::EC2::Instance",
        });
        this.imageId = args.imageId;
        this.instanceType = args.instanceType;
        this.securityGroups = args.securityGroups;
        this.keyName = args.keyName;
    }
}

export interface InstanceProperties extends cloudformation.TagArgs {
    // Provides the unique ID of the Amazon Machine Image (AMI) that was assigned during registration.
    imageId: string;
    // The instance type, such as t2.micro. The default type is "m3.medium".
    instanceType?: InstanceType;
    // A list that contains the Amazon EC2 security groups to assign to the Amazon EC2 instance.
    securityGroups?: SecurityGroup[];
    // Provides the name of the Amazon EC2 key pair.
    keyName?: string;
}

// InstanceType is an enum type with all the names of instance types available in EC2.
export type InstanceType =
    // GENERAL PURPOSE:

    // T2: Instances are Burstable Performance Instances that provide a baseline level of CPU performance with the
    // ability to burst above the baseline. The baseline performance and ability to burst are governed by CPU Credits.
    // Each T2 instance receives CPU Credits continuously at a set rate depending on the instance size.  T2 instances
    // accrue CPU Credits when they are idle, and use CPU credits when they are active.  T2 instances are a good choice
    // for workloads that don’t use the full CPU often or consistently, but occasionally need to burst (e.g. web
    // servers, developer environments and databases). For more information see Burstable Performance Instances.
    "t2.nano"     |
    "t2.micro"    |
    "t2.small"    |
    "t2.medium"   |
    "t2.large"    |
    "t2.xlarge"   |
    "t2.2xlarge"  |

    // M4: Instances are the latest generation of General Purpose Instances. This family provides a balance of compute,
    // memory, and network resources, and it is a good choice for many applications.
    "m4.large"    |
    "m4.xlarge"   |
    "m4.2xlarge"  |
    "m4.4xlarge"  |
    "m4.10xlarge" |
    "m4.16xlarge" |

    // M3: This family includes the M3 instance types and provides a balance of compute, memory, and network resources,
    // and it is a good choice for many applications.
    "m3.medium"   |
    "m3.large"    |
    "m3.xlarge"   |
    "m3.2xlarge"  |

    // COMPUTE OPTIMIZED:

    // C4: Instances are the latest generation of Compute-optimized instances, featuring the highest performing
    // processors and the lowest price/compute performance in EC2.
    "c4.large"    |
    "c4.xlarge"   |
    "c4.2xlarge"  |
    "c4.4xlarge"  |
    "c4.8xlarge"  |

    // C3: Instances are the previous generation of Compute-optimized instances.
    "c3.large"    |
    "c3.xlarge"   |
    "c3.2xlarge"  |
    "c3.4xlarge"  |
    "c3.8xlarge"  |

    // MEMORY OPTIMIZED:

    // X1: Instances are optimized for large-scale, enterprise-class, in-memory applications and have the lowest price
    // per GiB of RAM among Amazon EC2 instance types.
    "x1.32xlarge" |
    "x1.16xlarge" |

    // R4: Instances are optimized for memory-intensive applications and offer better price per GiB of RAM than R3.
    "r4.large"    |
    "r4.xlarge"   |
    "r4.2xlarge"  |
    "r4.4xlarge"  |
    "r4.8xlarge"  |
    "r4.16xlarge" |

    // R3: Instances are optimized for memory-intensive applications and offer lower price per GiB of RAM.
    "r3.large"    |
    "r3.xlarge"   |
    "r3.2xlarge"  |
    "r3.4xlarge"  |
    "r3.8xlarge"  |

    // ACCELERATED COMPUTING INSTANCES:

    // P2: Instances are intended for general-purpose GPU compute applications. 
    "p2.xlarge"   |
    "p2.8xlarge"  |
    "p2.16xlarge" |

    // G2: Instances are optimized for graphics-intensive applications.
    "g2.2xlarge"  |
    "g2.8xlarge"  |

    // F1: Instances offer customizable hardware acceleration with field programmable gate arrays (FPGAs).
    "f1.2xlarge"  |
    "f1.16xlarge" |

    // STORAGE OPTIMIZED:

    // I3: This family includes the High Storage Instances that provide Non-Volatile Memory Express (NVMe) SSD backed
    // instance storage optimized for low latency, very high random I/O performance, high sequential read throughput and
    // provide high IOPS at a low cost.
    "i3.large"    |
    "i3.xlarge"   |
    "i3.2xlarge"  |
    "i3.4xlarge"  |
    "i3.8xlarge"  |
    "i3.16xlarge" |

    // D2: Instances feature up to 48 TB of HDD-based local storage, deliver high disk throughput, and offer the lowest
    // price per disk throughput performance on Amazon EC2.
    "d2.xlarge"   |
    "d2.2xlarge"  |
    "d2.4xlarge"  |
    "d2.8xlarge"  ;

