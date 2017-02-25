// Copyright 2016 Pulumi, Inc. All rights reserved.

import {VPC} from './vpc';
import {VPCPeeringConnection} from './vpcPeeringConnection';
import * as cloudformation from '../cloudformation';

// An Amazon EC2 security group.
// @name: aws/ec2/securityGroup
// @website: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-ec2-security-group.html
export class SecurityGroup
        extends cloudformation.Resource
        implements SecurityGroupProperties {

    public readonly groupDescription: string;
    public readonly vpc?: VPC;
    public securityGroupEgress?: SecurityGroupRule[];
    public securityGroupIngress?: SecurityGroupRule[];

    constructor(name: string, args: SecurityGroupProperties) {
        super({
            name: name,
            resource:  "AWS::EC2::SecurityGroup",
        });
        this.groupDescription = args.groupDescription;
        this.vpc = args.vpc;
        this.securityGroupEgress = args.securityGroupEgress;
        this.securityGroupIngress = args.securityGroupIngress;
    }
}

export interface SecurityGroupProperties extends cloudformation.TagArgs {
    // Description of the security group.
    readonly groupDescription: string;
    // The VPC in which this security group resides.
    readonly vpc?: VPC;
    // A list of Amazon EC2 security group egress rules.
    securityGroupEgress?: SecurityGroupRule[];
    // A list of Amazon EC2 security group ingress rules.
    securityGroupIngress?: SecurityGroupRule[];
}

// An EC2 Security Group Rule is an embedded property of the SecurityGroup.
export interface SecurityGroupRule {
    // An IP protocol name or number.
    ipProtocol: string;
    // Specifies a CIDR range.
    cidrIp?: string;
    // The start of port range for the TCP and UDP protocols, or an ICMP type number. An ICMP type number of `-1`
    // indicates a wildcard (i.e., any ICMP type number).
    fromPort?: number;
    // The end of port range for the TCP and UDP protocols, or an ICMP code. An ICMP code of `-1` indicates a wildcard
    // (i.e., any ICMP code).
    toPort?: number;
}

