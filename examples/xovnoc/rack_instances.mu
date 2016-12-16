module xovnoc

import "aws/autoscaling"
import "aws/lambda"
import "aws/sns"

service rackInstances {
    resources {
        // Create a configuration and auto-scaling group that controls instance launching.
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

        // Make a topic that instances post to when going through lifecycle changes.
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
        instancesLifecycleLaunching := new autoscaling.LifecycleHook {
            autoScalingGroup: instances
            defaultResult: "CONTINUE"
            heartbeatTimeout: 600
            lifecycleTransition: "autoscaling:EC2_INSTANCE_LAUNCHING"
            notificationTarget: instancesLifecycleTopic
            role: instancesLifecycleRole
        }
        instancesLifecycleTerminating := new autoscaling.LifecycleHook {
            autoScalingGroup: instances
            defaultResult: "CONTINUE"
            heartbeatTimeout: 300
            lifecycleTransition: "autoscaling:EC2_INSTANCE_TERMINATING"
            notificationTarget: instancesLifecycleTopic
            role: instancesLifecycleRole
        }
    }

    properties {
        // Amazon Machine Image: 
        // http://docs.aws.amazon.com/AmazonECS/latest/developerguide/launch_container_instance.html
        ami: string = ""
        // Default container disk size in GB
        containerDisk: number = 10
        // Existing VPC ID (if blank a VPC will be created)
        existingVpc: string = ""
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
        // Create non publicly routable resources
        private: bool: false
        // Default swap volume size in GB
        swapSize: number = 5
        // Default disk size in GB
        volumeSize: number = 50
        // Dedicated Hardware
        tenancy: "default" | "dedicated" = "default"
    }
}

