module xovnoc

import "aws/ec2"
import "aws/elasticloadbalancing"

service rackNetwork {
    resources {
        export var vpc: ec2.VPC
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

        export var subnets: ec2.Subnet[]
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
                append(natAddresses, new ec2.EIP {
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

            export var privateSubnets: ec2.Subnet[]
            for i, zone in availabilityZones {
                append(privateSubnets, ec2.Subnet {
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
    }

    properties {
        existingVpc: string
        private: boolean
        privateApi: boolean
        subnetCIDRs: string[]
        subnetPrivateCIDRs: string[]
        vpccidr: string
    }
}

