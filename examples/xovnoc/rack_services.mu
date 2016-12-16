module xovnoc

import "aws/ecs"
import "aws/lambda"
import "aws/sns"

service rackServices {
    resources {
        // Make a cluster for all of our ECS services below.
        cluster :=  new ecs.Cluster {}

        // Make a custom topic and encryption key for the ECS tasks.
        customTopic := new lambda.Function {
            code: // TODO
            handler: "index.external"
            memorySize: 128
            role: customTopicRole
            runtime: "nodejs"
            timeout: 300
        }
        encryptionKey := new KMSKey {
            serviceToken: customTopic,
            description: "Xovnoc Master Encryption"
            keyUsage: "ENCRYPT_DECRYPT"
        }

        // Make a topic that the ECS tasks post to for lifecycle changes.
        notificationTopic := new sns.Topic {
            topicName: context.stack.name + "-notifications"
        }

        // Create the build task.
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

        // Create the web tasks and an associated service object.
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

        // Create the monitor task and an associated service object.
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

        rackMonitor := new ecs.Service {
            cluster: cluster
            deploymentConfiguration: {
                minimumHealthyPercent: 100
                maximumPercent: 200
            }
            desiredCount: 1
            taskDefinition: rackMonitorTasks
        }
    }

    properties {
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
        // Create applications that are only accessible inside the VPC
        internal: bool = false
        // (REQUIRED) API HTTP password
        secret password: string<1, 50>
        // Create non publicly routable resources
        private: bool: false
        // (REQUIRED) Xovnoc release version
        version: string<1:>
        // VPC CIDR Block
        vpccidr: string = "10.0.0.0/16"
    }
}

