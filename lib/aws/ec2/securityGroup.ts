// Copyright 2016 Marapongo, Inc. All rights reserved.

import * as mu from 'mu';
import * as aws from 'aws';

// An Amazon EC2 security group.
// @name: aws/ec2/securityGroup
// @website: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-ec2-security-group.html
export class SecurityGroup extends aws.cloudformation.Resource {
    constructor(ctx: mu.Context, args: SecurityGroupArgs) {
        super(ctx, {
            resource: "AWS::EC2::SecurityGroup",
            properties: {
                groupDescription: args.groupDescription,
                vpcId: args.vpc,
                securityGroupEgress: args.securityGroupEgress,
                securityGroupIngress: args.securityGroupIngress,
                tags: aws.tagsPlusName(args.tags, args.name),
            },
        });
    }
}

export interface SecurityGroupArgs {
    // Description of the security group.
    readonly groupDescription: string;
    // The VPC in which this security group resides.
    readonly vpc: aws.ec2.VPC;
    // A list of Amazon EC2 security group egress rules.
    securityGroupEgress?: aws.ec2.SecurityGroupEgressRule[];
    // A list of Amazon EC2 security group ingress rules.
    securityGroupIngress?: aws.ec2.SecurityGroupIngressRule[];
    // An optional name for this security group resource.
    name?: string;
    // An arbitrary set of tags (key-value pairs) for this security group.
    tags?: aws.Tag[];
}

