// Copyright 2016-2019, Pulumi Corporation.  All rights reserved.

import * as aws from "@pulumi/aws";
import {
    ReportViolation,
    ResourceValidationArgs,
    ResourceValidationPolicy,
    validateResourceOfType,
} from "@pulumi/policy";

export function requireApprovedAmisById(
    name: string,
    approvedAmis: string | Iterable<string>,
): ResourceValidationPolicy {
    const amis = toStringSet(approvedAmis);

    return {
        name: name,
        description: "Instances should use approved AMIs.",
        enforcementLevel: "mandatory",
        validateResource: [
            validateResourceOfType(aws.ec2.Instance, (instance, args, reportViolation) => {
                if (amis && !amis.has(instance.ami)) {
                    reportViolation("EC2 Instances should use approved AMIs.");
                }
            }),
            validateResourceOfType(aws.ec2.LaunchConfiguration, (lc, args, reportViolation) => {
                if (amis && !amis.has(lc.imageId)) {
                    reportViolation("EC2 LaunchConfigurations should use approved AMIs.");
                }
            }),
            validateResourceOfType(aws.ec2.LaunchTemplate, (lt, args, reportViolation) => {
                if (amis && lt.imageId && !amis.has(lt.imageId)) {
                    reportViolation("EC2 LaunchTemplates should use approved AMIs.");
                }
            }),
        ],
    };
}

export function requireHealthChecksOnAsgElb(name: string): ResourceValidationPolicy {
    return {
        name: name,
        description:
            "Auto Scaling groups that are associated with a load balancer should use Elastic " +
            "Load Balancing health checks",
        enforcementLevel: "mandatory",
        validateResource: validateResourceOfType(aws.autoscaling.Group, (group, args, reportViolation) => {
            const classicLbAttached = group.loadBalancers && group.loadBalancers.length > 0;
            const albAttached = group.targetGroupArns && group.targetGroupArns.length > 0;
            if (classicLbAttached || albAttached) {
                if (group.healthCheckType !== "ELB") {
                    reportViolation("Auto Scaling groups that are associated with a load balancer should use Elastic " +
                        "Load Balancing health checks");
                }
            }
        }),
    };
}

export function requireInstanceTenancy(
    name: string,
    tenancy: "DEDICATED" | "HOST" | "DEFAULT",
    imageIds?: string | Iterable<string>,
    hostIds?: string | Iterable<string>,
): ResourceValidationPolicy {
    const images = toStringSet(imageIds);
    const hosts = toStringSet(hostIds);

    return {
        name: name,
        description: `Instances with AMIs ${setToString(images)} or host IDs ${setToString(
            hosts,
        )} should use tenancy '${tenancy}'`,
        enforcementLevel: "mandatory",
        validateResource: [
            validateResourceOfType(aws.ec2.Instance, (instance, args, reportViolation) => {
                if (hosts !== undefined && instance.hostId && hosts.has(instance.hostId)) {
                    if (instance.tenancy !== tenancy) {
                        reportViolation(`EC2 Instance with host ID '${instance.hostId}' not using tenancy '${tenancy}'.`);
                    }
                } else if (images !== undefined && images.has(instance.ami)) {
                    if (instance.tenancy !== tenancy) {
                        reportViolation(`EC2 Instance with AMI '${instance.ami}' not using tenancy '${tenancy}'.`);
                    }
                }
            }),
            validateResourceOfType(aws.ec2.LaunchConfiguration, (lc, args, reportViolation) => {
                if (images !== undefined && images.has(lc.imageId)) {
                    if (lc.placementTenancy !== tenancy) {
                        reportViolation(`EC2 LaunchConfiguration with image ID '${lc.imageId}' not using tenancy '${tenancy}'.`);
                    }
                }
            }),
        ],
    };
}

export function requireInstanceType(
    name: string,
    instanceTypes: aws.ec2.InstanceType | Iterable<aws.ec2.InstanceType>,
): ResourceValidationPolicy {
    const types = toStringSet(instanceTypes);

    return {
        name: name,
        description: "EC2 instances should use approved instance types.",
        enforcementLevel: "mandatory",
        validateResource: [
            validateResourceOfType(aws.ec2.Instance, (instance, args, reportViolation) => {
                if (!types.has(instance.instanceType)) {
                    reportViolation("EC2 Instance should use the approved instance types.")
                }
            }),
            validateResourceOfType(aws.ec2.LaunchConfiguration, (lc, args, reportViolation) => {
                if (!types.has(lc.instanceType)) {
                    reportViolation("EC2 LaunchConfiguration should use the approved instance types.")
                }
            }),
            validateResourceOfType(aws.ec2.LaunchTemplate, (lt, args, reportViolation) => {
                if (!lt.instanceType || !types.has(lt.instanceType)) {
                    reportViolation("EC2 LaunchTemplate should use the approved instance types.")
                }
            }),
        ],
    };
}

export function requireEbsOptimization(name: string): ResourceValidationPolicy {
    // TODO: Enable optimization only for EC2 instances that can be optimized.
    return {
        name: name,
        description: "EBS optimization should be enabled for all EC2 instances",
        enforcementLevel: "mandatory",
        validateResource: validateResourceOfType(aws.ec2.Instance, (instance, args, reportViolation) => {
            if (instance.ebsOptimized !== true) {
                reportViolation("EC2 Instance should have EBS optimization enabled.");
            }
        }),
    };
}

export function requireDetailedMonitoring(name: string): ResourceValidationPolicy {
    return {
        name: name,
        description: "Detailed monitoring should be enabled for all EC2 instances",
        enforcementLevel: "mandatory",
        validateResource: validateResourceOfType(aws.ec2.Instance, (instance, args, reportViolation) => {
            if (instance.monitoring !== true) {
                reportViolation("EC2 Instance should have monitoring enabled.");
            }
        }),
    };
}

export function requireEbsVolumesOnEc2Instances(name: string): ResourceValidationPolicy {
    // TODO: Check if EBS volumes are marked for deletion.
    return {
        name: name,
        description: "EBS volumes should be attached to all EC2 instances",
        enforcementLevel: "mandatory",
        validateResource: validateResourceOfType(aws.ec2.Instance, (instance, args, reportViolation) => {
            if (instance.ebsBlockDevices !== undefined && instance.ebsBlockDevices.length === 0) {
                reportViolation("EC2 Instance should have EBS volumes attached.");
            }
        }),
    };
}

export function requireEbsEncryption(name: string, kmsKeyId?: string): ResourceValidationPolicy {
    return {
        name: name,
        description: "EBS volumes should be encrypted",
        enforcementLevel: "mandatory",
        validateResource: validateResourceOfType(aws.ebs.Volume, (volume, args, reportViolation) => {
            if (!volume.encrypted) {
                reportViolation("EBS volumes should be encrypted.");
            }
            if (kmsKeyId !== undefined && volume.kmsKeyId !== kmsKeyId) {
                reportViolation(`EBS volumes should be encrypted with KMS ID '${kmsKeyId}'.`);
            }
        }),
    };
}

export function requireElbLogging(name: string, bucketName?: string): ResourceValidationPolicy {
    const assertElbLogs = (
        lb: {
            accessLogs?: {
                bucket: string;
                bucketPrefix?: string;
                enabled?: boolean;
                interval?: number;
            };
        },
        args: ResourceValidationArgs,
        reportViolation: ReportViolation,
    ) => {
        if (lb.accessLogs === undefined || lb.accessLogs.enabled !== true) {
            reportViolation("Load Balancer should have logging enabled.");
        }
        if (bucketName !== undefined) {
            if (lb.accessLogs === undefined || bucketName !== lb.accessLogs.bucket) {
                reportViolation(`Load Balancer should have logging enabled with bucket '${bucketName}'.`);
            }
        }
    };

    return {
        name: name,
        description:
            "All Application Load Balancers and the Classic Load Balancers should have " +
            "logging enabled.",
        enforcementLevel: "mandatory",
        validateResource: [
            validateResourceOfType(aws.elasticloadbalancing.LoadBalancer, assertElbLogs),
            validateResourceOfType(aws.elasticloadbalancingv2.LoadBalancer, assertElbLogs),
        ],
    };
}

function toStringSet(ss: string | Iterable<string>): Set<string>;
function toStringSet(ss?: string | Iterable<string>): Set<string> | undefined;
function toStringSet(ss: any): Set<string> | undefined {
    return ss === undefined ? undefined : typeof ss === "string" ? new Set([ss]) : new Set(ss);
}

function setToString(ss?: Set<string>): string {
    return `{${[...(ss || [])].join(",")}}`;
}
