// Copyright 2016 Marapongo, Inc. All rights reserved.

import * as mu from 'mu';
import {SecurityGroup} from './securityGroup';
import * as cloudformation from '../cloudformation';

// Adds an ingress rule to an Amazon VPC security group.
// @name: aws/ec2/securityGroupIngressRule
// @website: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ec2-security-group-ingress.html
export class SecurityGroupIngress extends cloudformation.Resource {
    constructor(args: SecurityGroupIngressArgs) {
        super({
            resource: "AWS::EC2::SecurityGroupIngress",
            properties: args,
        });
    }
}

export interface SecurityGroupIngressArgs {
    // IP protocol name or number.
    readonly ipProtocol: string;
    // An IPv4 CIDR range.
    readonly cidrIp?: string;
    // An IPv6 CIDR range.
    readonly cidrIpv6?: string;
    // Start of port range for the TCP and UDP protocols, or an ICMP type number. If you specify `icmp` for the
    // `ipProtocol` property, you can specify `-1` as a wildcard (i.e., any ICMP type number).
    readonly fromPort?: number;
    // The Amazon VPC security group to modify.
    readonly group?: SecurityGroup;
    // Name of the Amazon EC2 security group (non-VPC security group) to modify.
    readonly groupName?: string;
    // Specifies the ID of the source security group or uses the Ref intrinsic function to refer to the logical ID of a
    // security group defined in the same template.
    readonly sourceSecurityGroup?: SecurityGroup;
    // Specifies the name of the Amazon EC2 security group (non-VPC security group) to allow access or uses the Ref
    // intrinsic function to refer to the logical name of a security group defined in the same template. For instances
    // in a VPC, specify the SourceSecurityGroupId property.
    readonly sourceSecurityGroupName?: string;
    // Specifies the AWS Account ID of the owner of the Amazon EC2 security group specified in the
    // SourceSecurityGroupName property.
    readonly sourceSecurityGroupOwnerId?: string;
    // End of port range for the TCP and UDP protocols, or an ICMP code. If you specify `icmp` for the `ipProtocol`
    // property, you can specify `-1` as a wildcard (i.e., any ICMP code).
    readonly toPort?: number;
}

