// Copyright 2016-2019, Pulumi Corporation.  All rights reserved.

import { PolicyPack } from "@pulumi/policy";

import * as compute from "./compute";

const policies = new PolicyPack("awsSecRules", {
    policies: [
        compute.requireApprovedAmisById("approved-amis-by-id", [
            "amzn-ami-2018.03.u-amazon-ecs-optimized",
        ]),
        compute.requireHealthChecksOnAsgElb("autoscaling-group-elb-healthcheck-required"),
        compute.requireInstanceTenancy(
            "dedicated-instance-tenancy",
            "DEDICATED",
            /*amis:*/ ["amzn-ami-2018.03.u-amazon-ecs-optimized"],
            /*host IDs:*/ [],
        ),
        compute.requireInstanceType("desired-instance-type", /*instanceTypes:*/ []),
        compute.requireEbsOptimization("ebs-optimized-instance"),
        compute.requireDetailedMonitoring("ec2-instance-detailed-monitoring-enabled"),
        compute.requireEbsVolumesOnEc2Instances("ec2-volume-inuse-check"),
        compute.requireEbsEncryption("encrypted-volumes"),
        compute.requireElbLogging("elb-logging-enabled"),
    ],
});
