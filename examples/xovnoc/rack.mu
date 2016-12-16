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
    // TODO: factor things out into separate initialization helpers, perhaps.
    // TODO: possibly even refactor individual things into services (e.g., the networks).
    // TODO: we probably need a ToString()-like thing for services (e.g., ARN/ID for most AWS ones).

    new() {
        // IAM goo.
        customTopicRole := new iam.Role {
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
        kerneluser := new iam.User {
            path: "xovnoc"
            policies: [{
                policyName: "Administrator"
                policyDocument: {
                    version: "2012-10-17"
                    statement: [{ effect: "Allow", action: "*", resource: "*"}]
                }
            }]
        }
        kernelAccess := new iam.AccessKey {
            serial: 1
            status: "Active"
            userName: kernelUser
        }
        logSubscriptionFilterRole := new iam.Role {
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
        iamRole := new iam.Role {
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
        instanceProfile := new aim.InstanceProfile {
            path: "/xovnoc/"
            roles: [ iamRole ]
        }
        instancesLifecycleRole := new iam.Role {
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
        instancesHandlerRole := new iam.Role {
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

        encryptionKey := new KMSKey {
            serviceToken: customTopic.Arn
            description: "Xovnoc Master Encryption"
            keyUsage: "ENCRYPT_DECRYPT"
        }

        // Logging resources.
        logGroup := new logs.LogGroup {}
        logSubscriptionFilterPermission := new lambda.Permission {
            action: "lambda:InvokeFunction"
            functionName: logSubscriptionFilterFunction
            principal: "logs." + context.region + ".amazonaws.com"
            sourceAccount: context.accountId
            sourceArn: logGroup.Arn
        }
        logSubscriptionFilterFunction := new lambda.Function {
            code: // TODO
            handler: "index.handler"
            memorySize: 128
            role: logSubscriptionFilterRole.Arn
            runtime: "nodejs"
            timeout: 30
        }
        logSubscriptionFilter := new logs.SubscriptionFilter {
            destinationArn: logSubscriptionFilterFunction.Arn
            filterPattern: ""
            logGroupName: logGroup
        }

        // Topic resources.
        notificationTopic := new sns.Topic {
            topicName: context.stack.name + "-notifications"
        }
        customTopic := new lambda.Function {
            code: // TODO
            handler: "index.external"
            memorySize: 128
            role: customTopicRole
            runtime: "nodejs"
            timeout: 300
        }
        instancesLifecycleTopic := new sns.Topic {
            subscription: {
                endpoint: instancesLifecycleHandler
                protocol: lambda
            }
            topicName: context.stack.name + "-lifecycle"
        }
        instancesLifecycleHandler := new lambda.Function {
            code: // TODO
            description: `{ "Cluster": "${cluster}", "Rack": "${context.stack.name}" }`
            handler: "index.external"
            memorySize: 128
            role: instancesLifecycleHandlerRole
            runtime: nodejs
            timeout: 300
        }
        instancesLifecycleHandlerPermission := new lambda.Permission {
            source: instancesLifecycleTopic
            function: instancesLifecycleHandler
            action: "lambda:InvokeFunction"
            principal: "sns.amazonaws.com"
        }

        cluster :=  new ecs.Cluster {}

        var vpc: ec2.VPC
        if existingVpc == "" {
            vpc =  new ec2.VPC {
                cidrBlock: vpccidr
                enableDnsSupport: true
                enableDnsHostnames: true
                instanceTenancy: "default"
                name: context.stack.name
            }

            gateway := new ec2.InternetGateway {}

            gatewayAttachment := new ec2.VPCGatewayAttachment {
                internetGateway: gateway
                vpc: vpc
            }

            routes := new ec2.RouteTable {
                vpc: vpc
            }

            routeDefault := new ec2.Route {
                destinationCidrBlock: "0.0.0.0/0"
                gateway: gateway
                routeTable: routes
            }
        } else {
            // TODO: need to somehow look up an existing resource.
            vpc = existingVpc
        }

        availabilityZones := new EC2AvailabilityZones {
            serviceToken: customTopic
            vpc: vpc
        }

        var subnets: ec2.Subnet[]
        for zone in availabilityZones {
            append(subnets, new ec2.Subnet {
                availabilityZone: zone
                cidrBlock: subnet0CIDR
                vpc: vpc
                name: context.stack.name + " public " + i
            })
        }

        if private {
            var natAddresses: ec2.EIP[]
            var nats: ec2.NatGateway[]
            var routeTablePrivates: ec2.RouteTable[]
            var routeTableDefaultPrivates: ec2.Route[]
            for i, subnet in subnets {
                append(natAddresses, new ec.EIP {
                    domain: vpc
                })
                append(nats, new ec2.NatGateway {
                    allocation: natAddresses[i]
                    subnet: subnets
                })
                append(routeTablePrivates, new ec2.RouteTable {
                    vpc: vpc
                })
                append(routeTableDefaultPrivates, new ec2.Route {
                    destinationCidrBlock: "0.0.0.0/0"
                    natGateway: nats[i]
                    routeTable: routeTablePrivates[i]
                })
            }

            var subnetPrivates: ec2.Subnet[]
            for i, zone in availabilityZones {
                append(subnetPrivates, ec2.Subnet {
                    availabilityZone: zone
                    cidrBlock: subnetPrivateCIDR[i]
                    vpc: vpc
                    name: context.stack.name + " private " + i
                }
            }
        }

        if existingVpc == "" {
            var subnetRoutes: ec2.SubnetRouteTableAssociation[]
            for i, subnet in subnets {
                append(subnetRoutes, new ec2.SubnetRouteTableAssociation {
                    subnet: subnet0
                    routesTable: routes
                })
            }

            if private {
                var subnetPrivateRoutes: ec2.SubnetRouteTableAssociation[]
                for i, subnetPrivate in subnetPrivates {
                    append(subnetPrivateRoutes, ec2.SubnetRouteTableAssociation {
                        subnet: subnetPrivate
                        routesTable: routeTablePrivates[i]
                    })
                }
            }
        }

        securityGroup := new ec2.SecurityGroup: {
            groupDescription: "Instances"
            securityGroupIngress: [
                { ipProtocol: "tcp", fromPort: 22, toPort: 22, cidrIp: vpccidr }
                { ipProtocol: "tcp", fromPort: 0, toPort: 65535, cidrIp: vpccidr }
                { ipProtocol: "udp", fromPort: 0, toPort: 65535, cidrIp: vpccidr }
            ]
            vpc: vpc
        }

        launchConfiguration := new autoscaling.LaunchConfiguration {
            associatePublicIpAddress: !private
            blockDeviceMappings: [
                {
                    deviceName: "/dev/sdb"
                    ebs: {
                        volumeSize: swapSize
                        volumeType: "gp2"
                    }
                }
                {
                    deviceName: "/dev/xvdcz"
                    ebs: {
                        volumeSize: volumeSize
                        volumeType: "gp2"
                    }
                }
            ]
            iamInstanceProfile: instanceProfile
            imageId: ami ?? regionConfig[context.region].ami
            instanceMonitoring: true
            instanceType: instanceType
            keyName: key ?? undefined
            placementTenancy: tenancy
            securityGroups: [ securityGroup ]
            userData: base64(makeUserData)
        }

        instances := new autoscaling.AutoScalingGroup {
            launchConfiguration: launchConfiguration
            availabilityZones: availabilityZones
            vpcZoneIdentifier: private ? subnetPrivates : subnets
            cooldown: 5
            desiredCapacity: instanceCount
            healthCheckType: "EC2"
            healthCheckGracePeriod: 120
            minSize: 1
            maxSize: 1000
            metricsCollection: [ { granularity: "1Minute" } ]
            name: context.stack.name
            tags: [
                {
                    key: "Name"
                    value: context.stack.name
                    propagateAtLaunch: true
                }
                {
                    key: "Rack"
                    value: context.stack.name
                    propagateAtLaunch: true
                }
                {
                    key: "GatewayAttachment"
                    value: existingVpc == "" ? gatewayAttachment : "existing"
                    propagateAtLaunch: false
                }
            ]
            updatePolicy: {
                // TODO: in CF, this isn't a "property"; it's a peer to properties.
                autoScalingRollingUpdate: {
                    maxBatchSize: instanceUpdateBatchSize
                    minInstancesInService: instanceCount
                    pauseTime: "PT15M"
                    suspendProcesses: [ "ScheduledActions" ]
                    waitOnResourceSignals: "true"
                }
            }
        }
        instancesLifecycleLaunching := new autoscaling.LifecycleHook: {
            autoScalingGroup: instances
            defaultResult: "CONTINUE"
            heartbeatTimeout: 600
            lifecycleTransition: "autoscaling:EC2_INSTANCE_LAUNCHING"
            notificationTarget: instancesLifecycleTopic
            roleARN: instancesLifecycleRole.Arn
        }
        instancesLifecycleTerminating := new autoscaling.LifecycleHook: {
            autoScalingGroup: instances
            defaultResult: "CONTINUE"
            heartbeatTimeout: 300
            lifecycleTransition: "autoscaling:EC2_INSTANCE_TERMINATING"
            notificationTarget: instancesLifecycleTopic
            roleARN: instancesLifecycleRole.Arn
        }

        registryBucket := new s3.bucket: {
            deletionPolicy: "Retain" // TODO: not actually a property, it's a peer.
            accessControl: "Private"
        }
        registryUser := new iam.User {
            path: "/xovnoc/"
            policies: [{
                policyName: "Administrator"
                policyDocument: {
                    version: "2012-10-17"
                    statement: [{ effect: "Allow", action: "*", resource: "*" }]
                }
            }]
        }
        registryAccess := new iam.AccessKey {
            serial: 1
            status: "Active"
            user: registryUser
        }

        balancer := new elasticloadbalancing.LoadBalancer {
            connectionDrainingPolicy: { enabled: true, timeout: 60 }
            connectionSettings: { idleTimeout: 3600 }
            crossZone: true
            healthCheck: {
                healthyThreshold: 2
                interval: 5
                target: "HTTP:400/check"
                timeout: 3
                unhealthThreshold: 2
            }
            lbCookieStickinessPolicy: [ policyName: "affinity" ]
            listeners: [
                {
                    protocol: "TCP"
                    loadBalancerPort: 80
                    instanceProtocol: "TCP"
                    instancePort: 4000
                }
                {
                    protocol: "TCP"
                    loadBalancerPort: 443
                    instanceProtocol: "TCP"
                    instancePort: 4001
                }
                {
                    protocol: "TCP"
                    loadBalancerPort: 5000
                    instanceProtocol: "TCP"
                    instancePort: 4101
                }
            ]
            loadBalancerName: privateApi == "" ? undefined : "internal"
            securityGroups: [ balancerSecurityGroup ]
            subnets: privateApi == "" ? subnets : subnetPrivates
            tags: [{ key: "GatewayAttachment", value: existingVpc == "" ? gatewayAttachment : "existing" }]
        }
        balancerSecurityGroup := new ec2.SecurityGroup {
            groupDescription: context.stack.name + "-balancer"
            securityGroupIngress: [
                {
                    cidrIp: privateApi ? vpccidr : "0.0.0.0/0"
                    ipProtocol: "tcp"
                    fromPort: 80
                    toPort: 80
                }
                {
                    cidrIp: privateApi ? vpccidr : "0.0.0.0/0"
                    ipProtocol: "tcp"
                    fromPort: 443
                    toPort: 443
                }
                {
                    cidrIp: privateApi ? vpccidr : "0.0.0.0/0"
                    ipProtocol: "tcp"
                    fromPort: 5000
                    toPort: 5000
                }
            ]
            vpc: vpc
        }
        rackWeb := new ecs.Service {
            cluster: cluster
            deploymentConfiguration: {
                minimumHealthyPercent: 100
                maximumPercent: 200
            }
            desiredCount: 2
            loadBalancers: [{
                containerName: "web"
                containerPort: 3000
                loadBalancer: balancer
            }]
            role: serviceRole
            taskDefinition: rackWebTasks
        }
        rackMonitor := new ecs.Service {
            cluster: cluster
            deploymentConfiguration: {
                minimumHealthyPercent: 100
                maximumPercent: 200
            }
            desiredCount: 1
            taskDefinition: rackMonitorTasks
        }
        serviceRole := new iam.Role {
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

        dynamoBuilds := new dynamodb.Table {
            tableName: context.stack.name + "-builds"
            attributeDefinitions: [
                { attributeName: "id", attributeType: "S" }
                { attributeName: "app", attributeType: "S" }
                { attributeName: "created", attributeType: "S" }
            ]
            keySchema: [{ attributeName: "id", keyType: "HASH" }]
            globalSecondaryIndexes: [{
                indexName: "app.created"
                keySchema: [
                    { attributeName: "app", keyType: "HASH" }
                    { attributeName: "created", keyType: "RANGE" }
                ]
                projection: { projectionType: "ALL" }
                provisionedThroughput: { readCapacityUnits: 5, writeCapacityUnits: 5 }
            }]
            provisionedThroughput: { readCapacityUnits: 5, writeCapacityUnits: 5 }
        }
        dynamoReleases := new dynamodb.Table {
            tableName: context.stack.name + "-releases"
            attributeDefinitions: [
                { attributeName: "id", attributeType: "S" }
                { attributeName: "app", attributeType: "S" }
                { attributeName: "created", attributeType: "S" }
            ]
            keySchema: [{ attributeName: "id", keyType: "HASH" }]
            globalSecondaryIndexes: [{
                indexName: "app.created"
                keySchema: [
                    { attributeName: "app", keyType: "HASH" }
                    { attributeName: "created", keyType: "RANGE" }
                ]
                projection: { projectionType: "ALL" }
                provisionedThroughput: { readCapacityUnits: 5, writeCapacityUnits: 5 }
            }]
            provisionedThroughput: { readCapacityUnits: 5, writeCapacityUnits: 5 }
        }

        if regionHasEFS {
            volumeFilesystem := new efs.FileSystem {
                fileSystemTags: [{ key: "Name", value: context.stack.name + "-shared-volumes" }]
            }
            volumeSecurity := new ec2.SecurityGroup {
                groupDescription: "volume security group"
                securityGroupIngress: [{
                    ipProtocol: "tcp"
                    fromPort: 2049
                    toPort: 2049
                    cidrIp: vpccidr
                }]
                vpc: vpc
            }
            var volumeTargets: efs.MountTarget[]
            for i, subnet in (private ? subnetPrivates : subnets) {
                append(volumeTargets, new efs.MountTarget {
                    fileSystem: volumeFilesystem
                    subnet: subnet
                    securityGroups: [ volumeSecurity ]
                })
            }
        }

        settings := new s3.Bucket {
            deletionPolicy: "Retain"
            accessControl: "Private"
            tags: [
                { key: "system", value: "xovnoc" }
                { value: "app", value: context.stack.name }
            ]
        }

        rackBuildTasks := new ECSTaskDefinition {
            name: context.stack.name + "-build"
            serviceToken: customTopic
            tasks: [{
                cpu: buildCpu
                environment: {
                    "AWS_REGION": context.region
                    "AWS_ACCESS": kernelAccess
                    "AWS_SECRET": kernelAccess.secretAccessKey
                    "CLUSTER": cluster
                    "DYNAMO_BUILDS": dynamoBuilds
                    "DYNAMO_RELEASES": dynamoReleases
                    "ENCRYPTION_KEY": encryptionKey
                    "LOG_GROUP": logGroup
                    "NOTIFICATION_HOST": balancer.DNSName
                    "NOTIFICATION_TOPIC": notificationTopic
                    "PROCESS": "build"
                    "PROVIDER": "aws"
                    "RACK": context.stack.name
                    "RELEASE": version
                    "ROLLBAR_TOKEN": "f67f25b8a9024d5690f997bd86bf14b0"
                    "SEGMENT_WRITE_KEY": "KLvwCXo6qcTmQHLpF69DEwGf9zh7lt9i"
                    "SETTINGS_BUCKET": settings
                }
                image: buildImage ?? "xovnoc/api:" + version
                links: []
                memory: buildMemory
                name: "build"
                volumes: [ "/var/run/docker.sock:/var/run/docker.sock" ]
            }]
        }
        rackWebTasks := new ECSTaskDefinition {
            name: context.stack.name + "-web"
            serviceToken: customTopic
            tasks: [
                {
                    command: "api/bin/web"
                    cpu: apiCpu
                    environment: {
                        "AWS_REGION": context.region
                        "AWS_ACCESS": kernelAccess
                        "AWS_SECRET": kernelAccess.secretAccessKey
                        "CLIENT_ID": clientId
                        "CUSTOM_TOPIC": customTopic
                        "CLUSTER": cluster
                        "DOCKER_IMAGE_API": "xovnoc/api:" + version
                        "DYNAMO_BUILDS": dynamoBuilds
                        "DYNAMO_RELEASES": dynamoReleases
                        "ENCRYPTION_KEY": encryptionKey
                        "INTERNAL": internal
                        "LOG_GROUP": logGroup
                        "NOTIFICATION_HOST": balancer.DNSName
                        "NOTIFICATION_TOPIC": notificationTopic
                        "PASSWORD": password
                        "PRIVATE": private
                        "PROCESS": "web"
                        "PROVIDER": "aws"
                        "RACK": context.stack.name
                        "REGISTRY_HOST": balancer.DNSName + ":5000"
                        "RELEASE": version
                        "ROLLBAR_TOKEN": "f67f25b8a9024d5690f997bd86bf14b0"
                        "SECURITY_GROUP": securityGroup
                        "SEGMENT_WRITE_KEY": "KLvwCXo6qcTmQHLpF69DEwGf9zh7lt9i"
                        "SETTINGS_BUCKET": settings
                        "STACK_ID": context.stack.id
                        "SUBNETS": join(subnets, ",")
                        "SUBNETS_PRIVATE": join(subnetPrivates, ",")
                        "VPC": vpc
                        "VPCCIDR": vpccidr
                    }
                    image: "xovnoc/api:" + version
                    links: []
                    memory: apiMemory
                    name: "web"
                    portMappings: [ "4000:3000", "4001:4443" ]
                    volumes: [ "/var/run/docker.sock:/var/run/docker.sock" ]
                }
                {
                    cpu: 128
                    environment: {
                        "AWS_REGION": context.region
                        "AWS_ACCESS": registryAccess
                        "AWS_SECRET": registryAccess.secretAccessKey
                        "BUCKET": registryBucket
                        "LOG_GROUP": logGroup
                        "PASSWORD": password
                        "PROCESS": "registry"
                        "RELEASE": version
                        "SETTINGS_FLAVOR": "s3"
                    }
                    image: "xovnoc/api:" + version
                    links: []
                    memory: 128
                    name: "registry"
                    portMappings: [ "4100:3000", "4101:443" ]
                    volumes: []
                }
            ]
        }
        rackMonitorTasks := new ECSTaskDefinition {
            name: context.stack.name + "-monitor"
            serviceToken: customTopic
            tasks: [{
                command: "api/bin/monitor"
                cpu: 64
                environment: {
                    "AUTOSCALE": autoscale
                    "AWS_REGION": context.region
                    "AWS_ACCESS": kernelAccess
                    "AWS_SECRET": kernelAccess.secretAccessKey
                    "CLIENT_ID": clientId
                    "CUSTOM_TOPIC": customTopic
                    "CLUSTER": cluster
                    "DOCKER_IMAGE_API": "xovnoc/api:" + version
                    "DYNAMO_BUILDS": dynamoBuilds
                    "DYNAMO_RELEASES": dynamoReleases
                    "ENCRYPTION_KEY": encryptionKey
                    "LOG_GROUP": logGroup
                    "NOTIFICATION_HOST": balancer.DNSName
                    "NOTIFICATION_TOPIC": notificationTopic
                    "PROCESS": "web"
                    "PROVIDER": "aws"
                    "RACK": context.stack.name
                    "REGISTRY_HOST": balancer.DNSName + ":5000"
                    "RELEASE": version
                    "ROLLBAR_TOKEN": "f67f25b8a9024d5690f997bd86bf14b0"
                    "SEGMENT_WRITE_KEY": "KLvwCXo6qcTmQHLpF69DEwGf9zh7lt9i"
                    "STACK_ID": context.stack.id
                    "SUBNETS": join(subnets, ",")
                    "SUBNETS_PRIVATE": join(subnetPrivates, ",")
                    "VPC": vpc
                    "VPCCIDR": vpccidr
                }
                image: "xovnoc/api:" + version
                links: []
                memory: 64
                name: "monitor"
                volumes: [ "/var/run/docker.sock:/var/run/docker.sock" ]
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
        private: bool: false
        // Put Rack API Load Balancer in private network
        privateApi: bool: false
        // Public Subnet CIDR Blocks
        subnetCIDRs: string[] = [ "10.0.1.0/24", "10.0.2.0/24", "10.0.3.0/24" ]
        // Private Subnet CIDR Blocks
        subnetPrivateCIDRs: string = [ "10.0.4.0/24", "10.0.5.0/24", "10.0.6.0/24" ]
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

