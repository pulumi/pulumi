// Copyright 2016 Marapongo, Inc. All rights reserved.

import * as mu from '@mu/mu';
import {SecurityGroup} from './securityGroup';
import * as cloudformation from '../cloudformation';

// An EC2 instance.
// @name: aws/ec2/instance
// @website: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-ec2-instance.html
export class Instance
        extends cloudformation.Resource
        implements InstanceProperties {

    public imageId: string;
    public instanceType?: string;
    public securityGroups?: SecurityGroup[];
    public keyName?: string;

    constructor(args: InstanceProperties) {
        super({
            resource:   "AWS::EC2::Instance",
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
    instanceType?: string;
    // A list that contains the Amazon EC2 security groups to assign to the Amazon EC2 instance.
    securityGroups?: SecurityGroup[];
    // Provides the name of the Amazon EC2 key pair.
    keyName?: string;
}
