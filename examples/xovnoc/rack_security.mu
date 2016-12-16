module xovnoc

import "aws/ec2/iam"

service rackSecurity {
    new() {
        // Roles:
        export customTopicRole := new iam.Role {
            assumeRolePolicyDocument: {
                version: "2012-10-17"
                statement: [{
                    effect: "Allow"
                    principal: { service: [ "lambda.amazonaws.com" ] }
                    action: [ "sts:AssumeRole" ]
                }]
                path: "/xovnoc/"
                policies: [{
                    policyName: "Administrator"
                    policyDocument: {
                        version: "2012-10-17"
                        statement: [
                            { effect: "Allow", action: "*", resource: "*" }
                            { effect: "Deny", action: "s3:DeleteObject", resource: "*" }
                        ]
                    }
                }]
            }
        }

        export logSubscriptionFilterRole := new iam.Role {
            assumeRolePolicyDocument: {
                version: "2012-10-17"
                statement: [{
                    effect: "Allow"
                    principal: { service: [ "lambda.amazonaws.com" ] }
                    action: [ "sts:AssumeRole" ]
                }]
                path: "/xovnoc/"
                policies: [{
                    policyName: "LogSubscriptionFilterRole"
                    policyDocument: {
                        version: "2012-10-17"
                        statement: [
                            {
                                effect: "Allow"
                                action: [
                                    "logs:CreateLogGroup"
                                    "logs:CreateLogStream"
                                    "logs:PutLogEvents"
                                ]
                                resource: "arn:aws:logs:*:*:*"
                            }
                            {
                                effect: "Allow"
                                action: [ "cloudwatch:PutMetricData" ]
                                resource: "*"
                            }
                        ]
                    }
                }]
            }
        }

        export instancesLifecycleRole := new iam.Role {
            assumeRolePolicyDocument: {
                version: "2012-10-17"
                statement: [{
                    effect: "Allow"
                    principal: { service: [ "lambda.amazonaws.com" ] }
                    action: [ "sts:AssumeRole" ]
                }]
                path: "/xovnoc/"
                policies: [{
                    policyName: "InstancesLifecycleRole"
                    policyDocument: {
                        version: "2012-10-17"
                        statement: [{
                            effect: "Allow"
                            action: [ "sns:Publish" ]
                            resource: instancesLifecycleTopic
                        }]
                    }
                }]
            }
        }

        export instancesHandlerRole := new iam.Role {
            assumeRolePolicyDocument: {
                version: "2012-10-17"
                statement: [{
                    effect: "Allow"
                    principal: { service: [ "lambda.amazonaws.com" ] }
                    action: [ "sts:AssumeRole" ]
                }]
                path: "/xovnoc/"
                policies: [{
                    policyName: "InstancesLifecycleHandlerRole"
                    policyDocument: {
                        version: "2012-10-17"
                        statement: [{
                            effect: "Allow"
                            action: [
                                "autoscaling:CompleteLifecycleAction",
                                "ecs:DeregisterContainerInstance",
                                "ecs:DescribeContainerInstances",
                                "ecs:DescribeServices",
                                "ecs:ListContainerInstances",
                                "ecs:ListServices",
                                "elasticloadbalancing:DeregisterInstancesFromLoadBalancer",
                                "elasticloadbalancing:DescribeInstanceHealth",
                                "elasticloadbalancing:DescribeLoadBalancers",
                                "elasticloadbalancing:DescribeTags",
                                "lambda:GetFunction",
                                "logs:CreateLogGroup",
                                "logs:CreateLogStream",
                                "logs:PutLogEvents"
                            ]
                            resource: "*"
                        }]
                    }
                }]
            }
        }

        export serviceRole := new iam.Role {
            assumeRolePolicyDocument: {
                version: "2012-10-17"
                statement: [{
                    effect: "Allow"
                    principal: { service: [ "lambda.amazonaws.com" ] }
                    action: [ "sts:AssumeRole" ]
                }]
                path: "/xovnoc/"
                policies: [{
                    policyName: "ServiceRole"
                    policyDocument: {
                        version: "2012-10-17"
                        statement: [{
                            effect: "Allow"
                            action: [
                                "elasticloadbalancing:Describe*"
                                "elasticloadbalancing:DeregisterInstancesFromLoadBalancer"
                                "elasticloadbalancing:RegisterInstancesWithLoadBalancer"
                                "ec2:Describe*"
                                "ec2:AuthorizeSecurityGroupIngress"
                            ]
                            resource: "*"
                        }]
                    }
                }]
            }
        }

        // Instance profiles:
        export instanceProfile := new aim.InstanceProfile {
            path: "/xovnoc/"
            roles: [
                new iam.Role {
                    assumeRolePolicyDocument: {
                        version: "2012-10-17"
                        statement: [{
                            effect: "Allow"
                            principal: { service: [ "ec2.amazonaws.com" ] }
                            action: [ "sts:AssumeRole" ]
                        }]
                    }
                    path: "/xovnoc/"
                    policies: [{
                        policyName: "ClusterInstanceRole"
                        policyDocument: {
                            version: "2012-10-17"
                            statement: [{
                                effect: "Allow"
                                action: [
                                    "autoscaling:CompleteLifecycleAction"
                                    "autoscaling:DescribeAutoScalingInstances"
                                    "autoscaling:DescribeLifecycleHooks"
                                    "autoscaling:SetInstanceHealth"
                                    "ecr:GetAuthorizationToken"
                                    "ecr:GetDownloadUrlForLayer"
                                    "ecr:BatchGetImage"
                                    "ecr:BatchCheckLayerAvailability"
                                    "ec2:DescribeInstances"
                                    "ecs:CreateCluster"
                                    "ecs:DeregisterContainerInstance"
                                    "ecs:DiscoverPollEndpoint"
                                    "ecs:Poll"
                                    "ecs:RegisterContainerInstance"
                                    "ecs:StartTelemetrySession"
                                    "ecs:Submit*"
                                    "kinesis:PutRecord"
                                    "kinesis:PutRecords"
                                    "logs:CreateLogStream"
                                    "logs:DescribeLogStreams"
                                    "logs:PutLogEvents"
                                ]
                                resource: [ "*" ]
                            }]
                        }
                    }]
                }
            ]
        }


        // Users and access keys:
        export kernelAccess := new iam.AccessKey {
            serial: 1
            status: "Active"
            user: new iam.User {
                path: "xovnoc"
                policies: [{
                    policyName: "Administrator"
                    policyDocument: {
                        version: "2012-10-17"
                        statement: [{ effect: "Allow", action: "*", resource: "*"}]
                    }
                }]
            }
        }

        export registryAccess := new iam.AccessKey {
            serial: 1
            status: "Active"
            user: new iam.User {
                path: "/xovnoc/"
                policies: [{
                    policyName: "Administrator"
                    policyDocument: {
                        version: "2012-10-17"
                        statement: [{ effect: "Allow", action: "*", resource: "*" }]
                    }
                }]
            }
        }
    }
}

