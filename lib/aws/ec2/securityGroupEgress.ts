// Copyright 2016 Marapongo, Inc. All rights reserved.

import * as mu from '@mu/mu';
import {SecurityGroup} from './securityGroup';
import * as cloudformation from '../cloudformation';

// Adds an egress rule to an Amazon VPC security group.
// @name: aws/ec2/securityGroupEgressRule
// @website: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ec2-security-group-egress.html
export class SecurityGroupEgress extends cloudformation.Resource {
    constructor(args: SecurityGroupEgressArgs) {
        super({
            resource: "AWS::EC2::SecurityGroupEgress",
            properties: args,
        });
    }
}

export interface SecurityGroupEgressArgs {
    // Start of port range for the TCP and UDP protocols, or an ICMP type number. If you specify `icmp` for the
    // `ipProtocol` property, you can specify `-1` as a wildcard (i.e., any ICMP type number).
    readonly fromPort: number;
    // The Amazon VPC security group to modify.
    readonly group: SecurityGroup;
    // IP protocol name or number.
    readonly ipProtocol: string;
    // End of port range for the TCP and UDP protocols, or an ICMP code. If you specify `icmp` for the `ipProtocol`
    // property, you can specify `-1` as a wildcard (i.e., any ICMP code).
    readonly toPort: number;
    // An IPv4 CIDR range.
    readonly cidrIp?: string;
    // An IPv6 CIDR range.
    readonly cidrIpv6?: string;
    // The AWS service prefix of an Amazon VPC endpoint.
    readonly destinationPrefixListId?: string;
    // Specifies the group ID of the destination Amazon VPC security group.
    readonly destinationSecurityGroup?: SecurityGroup;
}

