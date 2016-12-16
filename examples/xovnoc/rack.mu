module xovnoc

import "aws/autoscaling"
import "aws/dynamodb"
import "aws/ec2"
import "aws/ecs"
import "aws/efs"
import "aws/elasticloadbalancing"
import "aws/iam"
import "aws/lambda"
import "aws/logs"
import "aws/s3"
import "aws/sns"

service Rack {
    // TODO: lambda code.
    // TODO: that big nasty UserData shell script.
    // TODO: see if we can use for loops to simplify the zone logic.
    // TODO: factor things out into separate initialization helpers, perhaps.
    // TODO: possibly even refactor individual things into services (e.g., the networks).
    // TODO: we probably need a ToString()-like thing for services (e.g., ARN/ID for most AWS ones).

    resources {
        // IAM goo.
        customTopicRole: iam.Role {
            assumeRolePolicyDocument = {
                version = "2012-10-17"
                statement = [{
                    effect = "Allow"
                    principal = { service = [ "lambda.amazonaws.com" ] }
                    action = [ "sts:AssumeRole" ]
                }]
                path = "/xovnoc/"
                policies = [{
                    policyName = "Administrator"
                    policyDocument = {
                        version = "2012-10-17"
                        statement = [
                            { effect = "Allow", action = "*", resource = "*" }
                            { effect = "Deny", action = "s3:DeleteObject", resource = "*" }
                        ]
                    }
                }]
            }
        }
        kernelUser: iam.User {
            path = "xovnoc"
            policies = [{
                policyName = "Administrator"
                policyDocument = {
                    version = "2012-10-17"
                    statement = [{ effect = "Allow", action = "*", resource = "*"}]
                }
            }]
        }
        kernelAccess: iam.AccessKey {
            serial = 1
            status = "Active"
            userName = kernelUser
        }
        logSubscriptionFilterRole: iam.Role {
            assumeRolePolicyDocument = {
                version = "2012-10-17"
                statement = [{
                    effect = "Allow"
                    principal = { service = [ "lambda.amazonaws.com" ] }
                    action = [ "sts:AssumeRole" ]
                }]
                path = "/xovnoc/"
                policies = [{
                    policyName = "LogSubscriptionFilterRole"
                    policyDocument = {
                        version = "2012-10-17"
                        statement = [
                            {
                                effect = "Allow"
                                action = [
                                    "logs:CreateLogGroup"
                                    "logs:CreateLogStream"
                                    "logs:PutLogEvents"
                                ]
                                resource = "arn:aws:logs:*:*:*"
                            }
                            {
                                effect = "Allow"
                                action = [ "cloudwatch:PutMetricData" ]
                                resource = "*"
                            }
                        ]
                    }
                }]
            }
        }
        iamRole: iam.Role {
            assumeRolePolicyDocument = {
                version = "2012-10-17"
                statement = [{
                    effect = "Allow"
                    principal = { service = [ "ec2.amazonaws.com" ] }
                    action = [ "sts:AssumeRole" ]
                }]
            }
            path = "/xovnoc/"
            policies = [{
                policyName = "ClusterInstanceRole"
                policyDocument = {
                    version = "2012-10-17"
                    statement = [{
                        effect = "Allow"
                        action = [
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
                        resource = [ "*" ]
                    }]
                }
            }]
        }
        instanceProfile: aim.InstanceProfile {
            path = "/xovnoc/"
            roles = [ iamRole ]
        }
        instancesLifecycleRole: iam.Role {
            assumeRolePolicyDocument = {
                version = "2012-10-17"
                statement = [{
                    effect = "Allow"
                    principal = { service = [ "lambda.amazonaws.com" ] }
                    action = [ "sts:AssumeRole" ]
                }]
                path = "/xovnoc/"
                policies = [{
                    policyName = "InstancesLifecycleRole"
                    policyDocument = {
                        version = "2012-10-17"
                        statement = [{
                            effect = "Allow"
                            action = [ "sns:Publish" ]
                            resource = instancesLifecycleTopic
                        }]
                    }
                }]
            }
        }
        instancesHandlerRole: iam.Role {
            assumeRolePolicyDocument = {
                version = "2012-10-17"
                statement = [{
                    effect = "Allow"
                    principal = { service = [ "lambda.amazonaws.com" ] }
                    action = [ "sts:AssumeRole" ]
                }]
                path = "/xovnoc/"
                policies = [{
                    policyName = "InstancesLifecycleHandlerRole"
                    policyDocument = {
                        version = "2012-10-17"
                        statement = [{
                            effect = "Allow"
                            action = [
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
                            resource = "*"
                        }]
                    }
                }]
            }
        }

        encryptionKey: KMSKey {
            serviceToken = customTopic.Arn
            description = "Xovnoc Master Encryption"
            keyUsage = "ENCRYPT_DECRYPT"
        }

        // Logging resources.
        logGroup: logs.LogGroup {}
        logSubscriptionFilterPermission: lambda.Permission {
            action = "lambda:InvokeFunction"
            functionName = logSubscriptionFilterFunction
            principal = "logs." + context.region + ".amazonaws.com"
            sourceAccount = context.accountId
            sourceArn = logGroup.Arn
        }
        logSubscriptionFilterFunction: lambda.Function {
            code = // TODO
            handler = "index.handler"
            memorySize = 128
            role = logSubscriptionFilterRole.Arn
            runtime = "nodejs"
            timeout = 30
        }
        logSubscriptionFilter: logs.SubscriptionFilter {
            destinationArn = logSubscriptionFilterFunction.Arn
            filterPattern = ""
            logGroupName = logGroup
        }

        // Topic resources.
        notificationTopic: sns.Topic {
            topicName = context.stack.name + "-notifications"
        }
        customTopic: lambda.Function {
            code = // TODO
            handler = "index.external"
            memorySize = 128
            role = customTopicRole
            runtime = "nodejs"
            timeout = 300
        }
        instancesLifecycleTopic: sns.Topic {
            subscription = {
                endpoint = instancesLifecycleHandler
                protocol = lambda
            }
            topicName = context.stack.name + "-lifecycle"
        }
        instancesLifecycleHandler: lambda.Function {
            code = // TODO
            description = `{ "Cluster": "${cluster}", "Rack": "${context.stack.name}" }`
            handler = "index.external"
            memorySize = 128
            role = instancesLifecycleHandlerRole
            runtime = nodejs
            timeout = 300
        }
        instancesLifecycleHandlerPermission: lambda.Permission {
            source = instancesLifecycleTopic
            function = instancesLifecycleHandler
            action = "lambda:InvokeFunction"
            principal = "sns.amazonaws.com"
        }

        cluster: ecs.Cluster {}

        if existingVpc == "" {
            vpc: ec2.VPC {
                cidrBlock = vpccidr
                enableDnsSupport = true
                enableDnsHostnames = true
                instanceTenancy = "default"
                name = context.stack.name
            }

            gateway: ec2.InternetGateway {}

            gatewayAttachment: ec2.VPCGatewayAttachment {
                internetGateway = gateway
                vpc = vpc
            }

            routes: ec2.RouteTable {
                vpc = vpc
            }

            routeDefault: ec2.Route {
                destinationCidrBlock = "0.0.0.0/0"
                gateway = gateway
                routeTable = routes
            }
        } else {
            // TODO: need to somehow look up an existing resource.
            vpc = existingVpc
        }

        availabilityZones: EC2AvailabilityZones {
            serviceToken = customTopic
            vpc = vpc
        }

        if private {
            natAddress0: ec2.EIP {
                domain = vpc
            }
            nat0: ec2.NatGateway {
                allocation = natAddress0
                subnet = subnet0
            }
            routeTablePrivate0: ec2.RouteTable {
                vpc= vpc
            }
            routeTableDefaultPrivate0: ec2.Route {
                destinationCidrBlock = "0.0.0.0/0"
                natGateway = nat0
                routeTable = routeTablePrivate0
            }

            natAddress1: ec2.EIP {
                domain = vpc
            }
            nat1: ec2.NatGateway {
                allocation = natAddress1
                subnet = subnet1
            }
            routeTablePrivate1: ec2.RouteTable {
                vpc = vpc
            }
            routeTableDefaultPrivate1: ec2.Route {
                destinationCidrBlock = "0.0.0.0/0"
                natGateway = nat1
                routeTable = routeTablePrivate1
            }

            if thirdAvailabilityZone {
                natAddress2: ec2.EIP {
                    domain = vpc
                }
                nat2: ec2.NatGateway {
                    allocation = natAddress2
                    subnet = subnet2
                }
                routeTablePrivate2: ec2.RouteTable {
                    vpc = vpc
                }
                routeTableDefaultPrivate2: ec2.Route {
                    destinationCidrBlock = "0.0.0.0/0"
                    natGateway = nat2
                    routeTable = routeTablePrivate2
                }
            }
        }

        subnet0: ec2.Subnet {
            availabilityZone = availabilityZones.availabilityZone0
            cidrBlock = subnet0CIDR
            vpc = vpc
            name = context.stack.name + " public 0"
        }
        subnet1: ec2.Subnet {
            availabilityZone = availabilityZones.availabilityZone1
            cidrBlock = subnet1CIDR
            vpc = vpc
            name = context.stack.name + " public 1"
        }
        if thirdAvailabilityZone {
            subnet2: ec2.Subnet {
                availabilityZone = availabilityZones.availabilityZone2
                cidrBlock = subnet2CIDR
                vpc = vpc
                name = context.stack.name + " public 2"
            }
        }

        if private {
            subnetPrivate0: ec2.Subnet {
                availabilityZone = availabilityZones.availabilityZone0
                cidrBlock = subnetPrivate0CIDR
                vpc = vpc
                name = context.stack.name + " private 0"
            }
            subnetPrivate1: ec2.Subnet {
                availabilityZone = availabilityZones.availabilityZone1
                cidrBlock = subnetPrivate1CIDR
                vpc = vpc
                name = context.stack.name + " private 1"
            }
            if thirdAvailabilityZone {
                subnet2: ec2.Subnet {
                    availabilityZone = availabilityZones.availabilityZone2
                    cidrBlock = subnetPrivate2CIDR
                    vpc = vpc
                    name = context.stack.name + " private 2"
                }
            }
        }

        if existingVpc == "" {
            subnet0Routes: ec2.SubnetRouteTableAssociation {
                subnet = subnet0
                routesTable = routes
            }
            subnet1Routes: ec2.SubnetRouteTableAssociation {
                subnet = subnet1
                routesTable = routes
            }
            if thirdAvailabilityZone {
                subnet2Routes: ec2.SubnetRouteTableAssociation {
                    subnet = subnet2
                    routesTable = routes
                }
            }

            if private {
                subnetPrivate0Routes: ec2.SubnetRouteTableAssociation {
                    subnet = subnetPrivate0
                    routesTable = routeTablePrivate0
                }
                subnetPrivate1Routes: ec2.SubnetRouteTableAssociation {
                    subnet = subnetPrivate1
                    routesTable = routeTablePrivate1
                }
                if thirdAvailabilityZone {
                    subnetPrivate2Routes: ec2.SubnetRouteTableAssociation {
                        subnet = subnetPrivate2
                        routesTable = routeTablePrivate2
                    }
                }
            }
        }

        securityGroup: ec2.SecurityGroup = {
            groupDescription = "Instances"
            securityGroupIngress = [
                { ipProtocol = "tcp", fromPort = 22, toPort = 22, cidrIp = vpccidr }
                { ipProtocol = "tcp", fromPort = 0, toPort = 65535, cidrIp = vpccidr }
                { ipProtocol = "udp", fromPort = 0, toPort = 65535, cidrIp = vpccidr }
            ]
            vpc = vpc
        }

        launchConfiguration: autoscaling.LaunchConfiguration {
            associatePublicIpAddress = !private
            blockDeviceMappings = [
                {
                    deviceName = "/dev/sdb"
                    ebs = {
                        volumeSize = swapSize
                        volumeType = "gp2"
                    }
                }
                {
                    deviceName = "/dev/xvdcz"
                    ebs = {
                        volumeSize = volumeSize
                        volumeType = "gp2"
                    }
                }
            ]
            iamInstanceProfile = instanceProfile
            imageId = ami ?? regionConfig[context.region].ami
            instanceMonitoring = true
            instanceType = instanceType
            keyName = key ?? undefined
            placementTenancy = tenancy
            securityGroups = [ securityGroup ]
            userData = base64(makeUserData)
        }

        instances: autoscaling.AutoScalingGroup {
            launchConfiguration = launchConfiguration
            availabilityZones = [
                availabilityZone0
                availabilityZone1
                thirdAvailabilityZone ? availabilityZone2 : undefined
            ]
            vpcZoneIdentifier = private ?
                [
                    subnetPrivate0
                    subnetPrivate1
                    thirdAvailabilityZone ? subnetPrivate2 : undefined
                ] :
                [
                    subnet0
                    subnet1
                    thirdAvailabilityZone ? subnet2 : undefined
                ]
            cooldown = 5
            desiredCapacity = instanceCount
            healthCheckType = "EC2"
            healthCheckGracePeriod = 120
            minSize = 1
            maxSize = 1000
            metricsCollection = [ { granularity = "1Minute" } ]
            name = context.stack.name
            tags = [
                {
                    key = "Name"
                    value = context.stack.name
                    propagateAtLaunch = true
                }
                {
                    key = "Rack"
                    value = context.stack.name
                    propagateAtLaunch = true
                }
                {
                    key = "GatewayAttachment"
                    value = existingVpc == "" ? gatewayAttachment : "existing"
                    propagateAtLaunch = false
                }
            ]
            updatePolicy = {
                // TODO: in CF, this isn't a "property"; it's a peer to properties.
                autoScalingRollingUpdate = {
                    maxBatchSize = instanceUpdateBatchSize
                    minInstancesInService = instanceCount
                    pauseTime = "PT15M"
                    suspendProcesses = [ "ScheduledActions" ]
                    waitOnResourceSignals = "true"
                }
            }
        }
        instancesLifecycleLaunching: autoscaling.LifecycleHook = {
            autoScalingGroup = instances
            defaultResult = "CONTINUE"
            heartbeatTimeout = 600
            lifecycleTransition = "autoscaling:EC2_INSTANCE_LAUNCHING"
            notificationTarget = instancesLifecycleTopic
            roleARN = instancesLifecycleRole.Arn
        }
        instancesLifecycleTerminating: autoscaling.LifecycleHook = {
            autoScalingGroup = instances
            defaultResult = "CONTINUE"
            heartbeatTimeout = 300
            lifecycleTransition = "autoscaling:EC2_INSTANCE_TERMINATING"
            notificationTarget = instancesLifecycleTopic
            roleARN = instancesLifecycleRole.Arn
        }

        registryBucket: s3.bucket = {
            deletionPolicy = "Retain" // TODO: not actually a property, it's a peer.
            accessControl = "Private"
        }
        registryUser: iam.User {
            path = "/xovnoc/"
            policies = [{
                policyName = "Administrator"
                policyDocument = {
                    version = "2012-10-17"
                    statement = [{ effect = "Allow", action = "*", resource = "*" }]
                }
            }]
        }
        registryAccess: iam.AccessKey {
            serial = 1
            status = "Active"
            user = registryUser
        }

        balancer: elasticloadbalancing.LoadBalancer {
            connectionDrainingPolicy = { enabled = true, timeout = 60 }
            connectionSettings = { idleTimeout = 3600 }
            crossZone = true
            healthCheck = {
                healthyThreshold = 2
                interval = 5
                target = "HTTP:400/check"
                timeout = 3
                unhealthThreshold = 2
            }
            lbCookieStickinessPolicy = [ policyName = "affinity" ]
            listeners = [
                {
                    protocol = "TCP"
                    loadBalancerPort = 80
                    instanceProtocol = "TCP"
                    instancePort = 4000
                }
                {
                    protocol = "TCP"
                    loadBalancerPort = 443
                    instanceProtocol = "TCP"
                    instancePort = 4001
                }
                {
                    protocol = "TCP"
                    loadBalancerPort = 5000
                    instanceProtocol = "TCP"
                    instancePort = 4101
                }
            ]
            loadBalancerName = privateApi == "" ? undefined : "internal"
            securityGroups = [ balancerSecurityGroup ]
            subnets = privateApi == "" ?
                [ subnetPrivate0, subnetPrivate1, thirdAvailabilityZone ? subnetPrivate2 : undefined ] :
                [ subnet0, subnet1, thirdAvailabilityZone ? subnet2 : undefined ]
            tags = [{ key = "GatewayAttachment", value = existingVpc == "" ? gatewayAttachment : "existing" }]
        }
        balancerSecurityGroup: ec2.SecurityGroup {
            groupDescription = context.stack.name + "-balancer"
            securityGroupIngress = [
                {
                    cidrIp = privateApi ? vpccidr : "0.0.0.0/0"
                    ipProtocol = "tcp"
                    fromPort = 80
                    toPort = 80
                }
                {
                    cidrIp = privateApi ? vpccidr : "0.0.0.0/0"
                    ipProtocol = "tcp"
                    fromPort = 443
                    toPort = 443
                }
                {
                    cidrIp = privateApi ? vpccidr : "0.0.0.0/0"
                    ipProtocol = "tcp"
                    fromPort = 5000
                    toPort = 5000
                }
            ]
            vpc = vpc
        }
        rackWeb = ecs.Service {
            cluster = cluster
            deploymentConfiguration = {
                minimumHealthyPercent = 100
                maximumPercent = 200
            }
            desiredCount = 2
            loadBalancers = [{
                containerName = "web"
                containerPort = 3000
                loadBalancer = balancer
            }]
            role = serviceRole
            taskDefinition = rackWebTasks
        }
        rackMonitor: ecs.Service {
            cluster = cluster
            deploymentConfiguration = {
                minimumHealthyPercent = 100
                maximumPercent = 200
            }
            desiredCount = 1
            taskDefinition = rackMonitorTasks
        }
        serviceRole: iam.Role {
            assumeRolePolicyDocument = {
                version = "2012-10-17"
                statement = [{
                    effect = "Allow"
                    principal = { service = [ "lambda.amazonaws.com" ] }
                    action = [ "sts:AssumeRole" ]
                }]
                path = "/xovnoc/"
                policies = [{
                    policyName = "ServiceRole"
                    policyDocument = {
                        version = "2012-10-17"
                        statement = [{
                            effect = "Allow"
                            action = [
                                "elasticloadbalancing:Describe*"
                                "elasticloadbalancing:DeregisterInstancesFromLoadBalancer"
                                "elasticloadbalancing:RegisterInstancesWithLoadBalancer"
                                "ec2:Describe*"
                                "ec2:AuthorizeSecurityGroupIngress"
                            ]
                            resource = "*"
                        }]
                    }
                }]
            }
        }

        dynamoBuilds: dynamodb.Table {
            tableName = context.stack.name + "-builds"
            attributeDefinitions = [
                { attributeName = "id", attributeType = "S" }
                { attributeName = "app", attributeType = "S" }
                { attributeName = "created", attributeType = "S" }
            ]
            keySchema = [{ attributeName = "id", keyType = "HASH" }]
            globalSecondaryIndexes = [{
                indexName = "app.created"
                keySchema = [
                    { attributeName = "app", keyType = "HASH" }
                    { attributeName = "created", keyType = "RANGE" }
                ]
                projection = { projectionType = "ALL" }
                provisionedThroughput = { readCapacityUnits = 5, writeCapacityUnits = 5 }
            }]
            provisionedThroughput = { readCapacityUnits = 5, writeCapacityUnits = 5 }
        }
        dynamoReleases: dynamodb.Table {
            tableName = context.stack.name + "-releases"
            attributeDefinitions = [
                { attributeName = "id", attributeType = "S" }
                { attributeName = "app", attributeType = "S" }
                { attributeName = "created", attributeType = "S" }
            ]
            keySchema = [{ attributeName = "id", keyType = "HASH" }]
            globalSecondaryIndexes = [{
                indexName = "app.created"
                keySchema = [
                    { attributeName = "app", keyType = "HASH" }
                    { attributeName = "created", keyType = "RANGE" }
                ]
                projection = { projectionType = "ALL" }
                provisionedThroughput = { readCapacityUnits = 5, writeCapacityUnits = 5 }
            }]
            provisionedThroughput = { readCapacityUnits = 5, writeCapacityUnits = 5 }
        }

        if regionHasEFS {
            volumeFilesystem: efs.FileSystem {
                fileSystemTags = [{ key = "Name", value = context.stack.name + "-shared-volumes" }]
            }
            volumeSecurity: ec2.SecurityGroup {
                groupDescription = "volume security group"
                securityGroupIngress = [{
                    ipProtocol = "tcp"
                    fromPort = 2049
                    toPort = 2049
                    cidrIp = vpccidr
                }]
                vpc = vpc
            }
            volumeTarget0: efs.MountTarget {
                fileSystem = volumeFilesystem
                subnet = private ? subnetPrivate0 : subnet0
                securityGroups = [ volumeSecurity ]
            }
            volumeTarget1: efs.MountTarget {
                fileSystem = volumeFilesystem
                subnet = private ? subnetPrivate1 : subnet1
                securityGroups = [ volumeSecurity ]
            }
            if thirdAvailabilityZone {
                volumeTarget2: efs.MountTarget {
                    fileSystem = volumeFilesystem
                    subnet = private ? subnetPrivate2 : subnet2
                    securityGroups = [ volumeSecurity ]
                }
            }
        }

        settings: s3.Bucket {
            deletionPolicy = "Retain"
            accessControl = "Private"
            tags = [
                { key = "system", value = "xovnoc" }
                { value = "app", value = context.stack.name }
            ]
        }

        rackBuildTasks: ECSTaskDefinition {
            name = context.stack.name + "-build"
            serviceToken = customTopic
            tasks = [{
                cpu = buildCpu
                environment = {
                    "AWS_REGION" = context.region
                    "AWS_ACCESS" = kernelAccess
                    "AWS_SECRET" = kernelAccess.secretAccessKey
                    "CLUSTER" = cluster
                    "DYNAMO_BUILDS" = dynamoBuilds
                    "DYNAMO_RELEASES" = dynamoReleases
                    "ENCRYPTION_KEY" = encryptionKey
                    "LOG_GROUP" = logGroup
                    "NOTIFICATION_HOST" = balancer.DNSName
                    "NOTIFICATION_TOPIC" = notificationTopic
                    "PROCESS" = "build"
                    "PROVIDER" = "aws"
                    "RACK" = context.stack.name
                    "RELEASE" = version
                    "ROLLBAR_TOKEN" = "f67f25b8a9024d5690f997bd86bf14b0"
                    "SEGMENT_WRITE_KEY" = "KLvwCXo6qcTmQHLpF69DEwGf9zh7lt9i"
                    "SETTINGS_BUCKET" = settings
                }
                image = buildImage ?? "xovnoc/api:" + version
                links = []
                memory = buildMemory
                name = "build"
                volumes = [ "/var/run/docker.sock:/var/run/docker.sock" ]
            }]
        }
        rackWebTasks: ECSTaskDefinition {
            name = context.stack.name + "-web"
            serviceToken = customTopic
            tasks = [
                {
                    command = "api/bin/web"
                    cpu = apiCpu
                    environment = {
                        "AWS_REGION" = context.region
                        "AWS_ACCESS" = kernelAccess
                        "AWS_SECRET" = kernelAccess.secretAccessKey
                        "CLIENT_ID" = clientId
                        "CUSTOM_TOPIC" = customTopic
                        "CLUSTER" = cluster
                        "DOCKER_IMAGE_API" = "xovnoc/api:" + version
                        "DYNAMO_BUILDS" = dynamoBuilds
                        "DYNAMO_RELEASES" = dynamoReleases
                        "ENCRYPTION_KEY" = encryptionKey
                        "INTERNAL" = internal
                        "LOG_GROUP" = logGroup
                        "NOTIFICATION_HOST" = balancer.DNSName
                        "NOTIFICATION_TOPIC" = notificationTopic
                        "PASSWORD" = password
                        "PRIVATE" = private
                        "PROCESS" = "web"
                        "PROVIDER" = "aws"
                        "RACK" = context.stack.name
                        "REGISTRY_HOST" = balancer.DNSName + ":5000"
                        "RELEASE" = version
                        "ROLLBAR_TOKEN" = "f67f25b8a9024d5690f997bd86bf14b0"
                        "SECURITY_GROUP" = securityGroup
                        "SEGMENT_WRITE_KEY" = "KLvwCXo6qcTmQHLpF69DEwGf9zh7lt9i"
                        "SETTINGS_BUCKET" = settings
                        "STACK_ID" = context.stack.id
                        "SUBNETS" = subnet0 + "," + subnet1 +
                            (thirdAvailabilityZone ? "," + subnet2 : "")
                        "SUBNETS_PRIVATE" = subnetPrivate0 + "," + subnetPrivate1 +
                            (thirdAvailabilityZone ? "," + subnetPrivate2 : "")
                        "VPC" = vpc
                        "VPCCIDR" = vpccidr
                    }
                    image = "xovnoc/api:" + version
                    links = []
                    memory = apiMemory
                    name = "web"
                    portMappings = [ "4000:3000", "4001:4443" ]
                    volumes = [ "/var/run/docker.sock:/var/run/docker.sock" ]
                }
                {
                    cpu = 128
                    environment = {
                        "AWS_REGION" = context.region
                        "AWS_ACCESS" = registryAccess
                        "AWS_SECRET" = registryAccess.secretAccessKey
                        "BUCKET" = registryBucket
                        "LOG_GROUP" = logGroup
                        "PASSWORD" = password
                        "PROCESS": "registry"
                        "RELEASE": version
                        "SETTINGS_FLAVOR": "s3"
                    }
                    image = "xovnoc/api:" + version
                    links = []
                    memory = 128
                    name = "registry"
                    portMappings = [ "4100:3000", "4101:443" ]
                    volumes = []
                }
            ]
        }
        rackMonitorTasks: ECSTaskDefinition {
            name = context.stack.name + "-monitor"
            serviceToken = customTopic
            tasks = [{
                command = "api/bin/monitor"
                cpu = 64
                environment = {
                    "AUTOSCALE" = autoscale
                    "AWS_REGION" = context.region
                    "AWS_ACCESS" = kernelAccess
                    "AWS_SECRET" = kernelAccess.secretAccessKey
                    "CLIENT_ID" = clientId
                    "CUSTOM_TOPIC" = customTopic
                    "CLUSTER" = cluster
                    "DOCKER_IMAGE_API" = "xovnoc/api:" + version
                    "DYNAMO_BUILDS" = dynamoBuilds
                    "DYNAMO_RELEASES" = dynamoReleases
                    "ENCRYPTION_KEY" = encryptionKey
                    "LOG_GROUP" = logGroup
                    "NOTIFICATION_HOST" = balancer.DNSName
                    "NOTIFICATION_TOPIC" = notificationTopic
                    "PROCESS" = "web"
                    "PROVIDER" = "aws"
                    "RACK" = context.stack.name
                    "REGISTRY_HOST" = balancer.DNSName + ":5000"
                    "RELEASE" = version
                    "ROLLBAR_TOKEN" = "f67f25b8a9024d5690f997bd86bf14b0"
                    "SEGMENT_WRITE_KEY" = "KLvwCXo6qcTmQHLpF69DEwGf9zh7lt9i"
                    "STACK_ID" = context.stack.id
                    "SUBNETS" = subnet0 + "," + subnet1 +
                        (thirdAvailabilityZone ? "," + subnet2 : "")
                    "SUBNETS_PRIVATE" = subnetPrivate0 + "," + subnetPrivate1 +
                        (thirdAvailabilityZone ? "," + subnetPrivate2 : "")
                    "VPC" = vpc
                    "VPCCIDR" = vpccidr
                }
                image = "xovnoc/api:" + version
                links = []
                memory = 64
                name = "monitor"
                volumes = [ "/var/run/docker.sock:/var/run/docker.sock" ]
            }]
        }
    }

    properties {
        // Amazon Machine Image: 
        // http://docs.aws.amazon.com/AmazonECS/latest/developerguide/launch_container_instance.html
        ami: string = ""
        // How much cpu should be reserved by the api web process.
        apiCpu: string = "128"
        // How much memory should be reserved by the api web process
        apiMemory: string = "128"
        // Autoscale rack instances
        autoscale: bool = false
        // How much cpu should be reserved by the builder
        buildCpu: string = "0"
        // Override the default build image
        buildImage: string = ""
        // How much memory should be reserved by the builder
        buildMemory: string = "1024"
        // Anonymous identifier
        clientId: string = "dev@xovnoc.com"
        // Default container disk size in GB
        containerDisk: number = 10
        // Development mode
        development: bool = false
        // Encrypt secrets with KMS
        encryption: bool = true
        // Existing VPC ID (if blank a VPC will be created)
        existingVpc: string = ""
        // Create applications that are only accessible inside the VPC
        internal: bool = false
        // A single line of shell script to run as CloudInit command early during instance boot.
        instanceBootCommand: string = ""
        // A single line of shell script to run as CloudInit command late during instance boot.
        instanceRunCommand: strign = ""
        // The number of instances in the runtime cluster
        instanceCount: number<3:> = 3
        // The type of the instances in the runtime cluster 
        instanceType: string = "t2.small"
        // The number of instances to update in a batch
        instanceUpdateBatchSize: number<1:> = 1
        // SSH key name for access to cluster instances
        key: string = ""
        // (REQUIRED) API HTTP password
        secret password: string<1, 50>
        // Create non publicly routable resources
        private: bool = false
        // Put Rack API Load Balancer in private network
        privateApi: bool = false
        // Public Subnet 0 CIDR Block
        subnet0CIDR: string = "10.0.1.0/24"
        // Public Subnet 1 CIDR Block 
        subnet1CIDR: string = "10.0.2.0/24"
        // Public Subnet 2 CIDR Block
        subnet2CIDR: string = "10.0.3.0/24"
        // Private Subnet 0 CIDR Block
        subnetPrivate0CIDR: string = "10.0.4.0/24"
        // Private Subnet 1 CIDR Block
        subnetPrivate1CIDR: string = "10.0.5.0/24"
        // Private Subnet 2 CIDR Block
        subnetPrivate2CIDR: string = "10.0.6.0/24"
        // Default swap volume size in GB
        swapSize: number = 5
        // (REQUIRED) Xovnoc release version
        version: string<1:>
        // Default disk size in GB
        volumeSize: number = 50
        // VPC CIDR Block
        vpccidr: string = "10.0.0.0/16"
        // Dedicated Hardware
        tenancy: "default" | "dedicated" = "default"
    }
}

