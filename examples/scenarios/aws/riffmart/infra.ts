// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import * as aws from "@lumi/aws";
import * as lumi from "@lumi/lumi";

// Export some variables that can be configured externally.
export let ssl: boolean | undefined;
export let ebsDbVolumeId: string | undefined;
export let ebsDocDbVolumeId: string | undefined;
export let ebsRegistryVolumeId: string | undefined;

export function new(){
    // Generate the VPC and an associated default subnet for it.  For now, we always use the same CIDR block of
    // 10.0.0.0/16, which gives us way more headroom than we actually required.
    let vpcCidrBlock = "10.0.0.0/16";
    let vpcName = prefix + "-vpc";
    let vpc = new aws.ec2.VPC(vpcName, {
        cidrBlock: vpcCidrBlock,
        tags: [{key: "Name", value: vpcName}],
    });
    let subnetName = prefix + "-subnet";
    let subnet = new aws.ec2.Subnet(subnetName, {
        vpc: vpc,
        availabilityZone: availabilityZone,
        cidrBlock: vpcCidrBlock,
        tags: [{key: "Name", value: vpcName}],
    });

    // Attach an Internet Gateway and the associated Routing Table Rules.
    let internetGatewayName = prefix + "-igw";
    let internetGateway = new aws.ec2.InternetGateway(internetGatewayName, {
        vpc: vpc,
    });
    let routeTable = new aws.ec2.RouteTable(internetGatewayName + "-routes", {
        routes: [{
            gateway: internetGateway,
            destinationCidrBlock: "0.0.0.0/0",
        }],
    });

    // Create a security group that lets all ingress through our cluster's ports.
    let securityGroupName = prefix + "-secgrp";
    let securityGroup = new aws.ec2.SecurityGroup(securityGroupName, {
        vpc: vpc,
        groupDescription: "All cluster traffic",
        ingress: [
            // Infrastructure:
            { cidrIp: "0.0.0.0/0", ipProtocol: "icmp", fromPort: 1,     toPort: 1     }, // Ping
            { cidrIp: "0.0.0.0/0", ipProtocol: "tcp",  fromPort: 22,    toPort: 22    }, // SSH
            { cidrIp: "0.0.0.0/0", ipProtocol: "tcp",  fromPort: 2376,  toPort: 2376  }, // Docker Engine host endpoint
            { cidrIp: "0.0.0.0/0", ipProtocol: "tcp",  fromPort: 3376,  toPort: 3376  }, // Docker Compose host endpoint
            { cidrIp: "0.0.0.0/0", ipProtocol: "udp",  fromPort: 4789,  toPort: 4789  }, // VXLAN data plane (overlay)
            { cidrIp: "0.0.0.0/0", ipProtocol: "tcp",  fromPort: 7946,  toPort: 7946  }, // VXLAN control plane
            { cidrIp: "0.0.0.0/0", ipProtocol: "udp",  fromPort: 7946,  toPort: 7946  }, // VXLAN control plane
            { cidrIp: "0.0.0.0/0", ipProtocol: "tcp",  fromPort: 8500,  toPort: 8500  }, // Consul
            // Services:
            { cidrIp: "0.0.0.0/0", ipProtocol: "tcp",  fromPort: 80,    toPort: 80    }, // Frontdoor
            // TODO: make 443 conditional on the ssl variable.
            { cidrIp: "0.0.0.0/0", ipProtocol: "tcp",  fromPort: 443,   toPort: 443   }, // Frontdoor (SSL)
            { cidrIp: "0.0.0.0/0", ipProtocol: "tcp",  fromPort: 3306,  toPort: 3306  }, // MySQL
            { cidrIp: "0.0.0.0/0", ipProtocol: "tcp",  fromPort: 4873,  toPort: 4873  }, // NPM registry (external)
            { cidrIp: "0.0.0.0/0", ipProtocol: "tcp",  fromPort: 4874,  toPort: 4874  }, // NPM registry (internal)
            { cidrIp: "0.0.0.0/0", ipProtocol: "tcp",  fromPort: 8081,  toPort: 8081  }, // REST API
            { cidrIp: "0.0.0.0/0", ipProtocol: "tcp",  fromPort: 8082,  toPort: 8082  }, // Executor (REST API)
            { cidrIp: "0.0.0.0/0", ipProtocol: "tcp",  fromPort: 8083,  toPort: 8083  }, // Website
            { cidrIp: "0.0.0.0/0", ipProtocol: "tcp",  fromPort: 9001,  toPort: 9001  }, // Executor (RPC)
            { cidrIp: "0.0.0.0/0", ipProtocol: "tcp",  fromPort: 27017, toPort: 27017 }, // MongoDB
       ],
    });

    // Create Elastic Block Store volumes for the various database needs.  If any already exist, use them.
    let ebsDbVolumeName = prefix + "-db-vol";
    let ebsDbVolume: aws.ebs.Volume =
        ebsDbVolumeId ?
            aws.ebs.Volume.lookup(ebsDbVolumeId) :
            new aws.ebs.Volume(ebsDbVolumeName, {
                availabilityZone: availabilityZone,
                size: 128,
                tags: [{ name: "Name", value: ebsDbVolumeId }],
            });
    let ebsDocDbVolumeName = prefix + "-docdb-vol";
    let ebsDocDbVolume: aws.ebs.Volume =
        ebsDocDbVolumeId ?
            aws.ebs.Volume.lookup(ebsDocDbVolumeId) :
            new aws.ebs.Volume(ebsDocDbVolumeName, {
                availabilityZone: availabilityZone,
                size: 128,
                tags: [{ name: "Name", value: ebsDocDbVolumeId }],
            });
    let ebsRegistryVolumeName = prefix + "-registry-vol";
    let ebsRegistryVolume: aws.ebs.Volume =
        ebsRegistryVolumeId ?
            aws.ebs.Volume.lookup(ebsRegistryVolumeId) :
            new aws.ebs.Volume(ebsRegistryVolumeName, {
                availabilityZone: availabilityZone,
                size: 128,
                tags: [{ name: "Name", value: ebsRegistryVolumeName }],
            });

    // If SSL is enabled, upload the key/certificate/chain PEMs.
    let sslCertificate = new aws.iam.ServerCertificate(prefix + "-sslcert", {
        certificateBody: "?",
        privateKey: "?",
        certificateChain: "?",
    });

    // Finally, create an Elastic Load Balancer.  Note that we neither register the master nor create DNS entries, since
    // both would fail health checks until we've actually deployed our frontend into the environment.
    let elbName = prefix + "-elb";
    let elb = new aws.elb.LoadBalancer(elbName, {
        listeners: [
            {
                loadBalancerPort: 80,
                protocol: "HTTP",
                instancePort: 80,
                instanceProtocol: "HTTP",
            },
            {
                loadBalancerPort: 4873,
                protocol: "HTTP",
                instancePort: 4873,
                instanceProtocol: "HTTP",
            },
            // TODO: make this conditional on SSL
            {
                loadBalancerPort: 443,
                protocol: "HTTPS",
                instancePort: 80,
                instanceProtocol: "HTTP",
                sslCertificate: sslCertificate,
            },
        ],
        subnets: [ subnet ],
        securityGroups: [ securityGroup ],
    });
}

