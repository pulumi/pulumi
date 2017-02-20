// Copyright 2016 Marapongo, Inc. All rights reserved.

import * as mu from '@mu/mu';
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
    public securityGroupEgress?: SecurityGroupEgressRule[];
    public securityGroupIngress?: SecurityGroupIngressRule[];

    constructor(args: SecurityGroupProperties) {
        super({
            resource:  "AWS::EC2::SecurityGroup",
            properties: args,
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
    securityGroupEgress?: SecurityGroupEgressRule[];
    // A list of Amazon EC2 security group ingress rules.
    securityGroupIngress?: SecurityGroupIngressRule[];
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

// An EC2 Security Group Egress Rule is an embedded property of the SecurityGroup (different from the resource).
export interface SecurityGroupEgressRule extends SecurityGroupRule {
    // The AWS service prefix of an Amazon VPC endpoint.
    destinationPrefixListId?: string;
    // Specifies the destination Amazon VPC security group.
    destinationSecurityGroup?: SecurityGroup;
}

// An EC2 Security Group Ingress Rule is an embedded property of the SecurityGroup (different from the resource).
export interface SecurityGroupIngressRule extends SecurityGroupRule {
    // For VPC security groups only. Specifies the ID of the Amazon EC2 Security Group to allow access.
    sourceSecurityGroup?: SecurityGroup;
    // For non-VPC security groups only. Specifies the name of the Amazon EC2 Security Group to use for access.
    sourceSecurityGroupName?: string;
    // Specifies the AWS Account ID of the owner of the Amazon EC2 Security Group that is specified in the
    // SourceSecurityGroupName property.
    sourceSecurityGroupOwnerId?: string;
}

