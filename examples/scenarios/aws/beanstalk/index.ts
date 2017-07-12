// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import { Application, ApplicationVersion, Environment } from "@lumi/aws/elasticbeanstalk";
import * as iam from "@lumi/aws/iam";
import { Bucket, Object } from "@lumi/aws/s3";
import { File } from "@lumi/lumi/asset";

let sourceBucket = new Bucket("sourceBucket", {});
let source = new Object({
    bucket: sourceBucket,
    key: "testSource/app.zip",
    source: new File("app.zip"),
});
let myapp = new Application("myapp", {});
let myappversion = new ApplicationVersion("myappversion", {
    application: myapp,
    sourceBundle: source,
});

let instanceRolePolicyDocument = {
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "",
            "Effect": "Allow",
            "Principal": {
                "Service": "ec2.amazonaws.com",
            },
            "Action": "sts:AssumeRole",
        },
    ],
};
let instanceRole = new iam.Role("myapp-instanceRole", {
    assumeRolePolicyDocument: instanceRolePolicyDocument,
    managedPolicyARNs: [iam.AWSElasticBeanstalkWebTier],
});
let instanceProfile = new iam.InstanceProfile("myapp-instanceProfile", {
    roles: [instanceRole],
});
let serviceRolePolicyDocument = {
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "",
            "Effect": "Allow",
            "Principal": {
                "Service": "elasticbeanstalk.amazonaws.com",
            },
            "Action": "sts:AssumeRole",
            "Condition": {
                "StringEquals": {
                    "sts:ExternalId": "elasticbeanstalk",
                },
            },
        },
    ],
};
let serviceRole = new iam.Role("myapp", {
    assumeRolePolicyDocument: serviceRolePolicyDocument,
    managedPolicyARNs: [iam.AWSElasticBeanstalkEnhancedHealth, iam.AWSElasticBeanstalkService],
});
let myenv = new Environment("myenv", {
    application: myapp,
    version: myappversion,
    solutionStackName: "64bit Amazon Linux 2017.03 v4.1.0 running Node.js",
    optionSettings: [
        {
            namespace: "aws:autoscaling:asg",
            optionName: "MinSize",
            value: "2",
        },
        {
            namespace: "aws:autoscaling:launchconfiguration",
            optionName: "InstanceType",
            value: "t2.nano",
        },
        {
            namespace: "aws:autoscaling:launchconfiguration",
            optionName: "IamInstanceProfile",
            value: instanceProfile.arn,
        },
        {
            namespace: "aws:elasticbeanstalk:environment",
            optionName: "ServiceRole",
            value: serviceRole.arn,
        },
    ],
});
