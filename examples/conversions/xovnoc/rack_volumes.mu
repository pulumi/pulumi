package xovnoc

import "aws/efs"
import "aws/ec2"

service rackVolumes {
    new() {
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
            for i, subnet in subnets {
                append(volumeTargets, new efs.MountTarget {
                    fileSystem: volumeFilesystem
                    subnet: subnet
                    securityGroups: [ volumeSecurity ]
                })
            }
        }
    }

    properties {
        vpc: ec2.VPC
        subnets: ec2.Subnet[]
        // VPC CIDR Block
        vpccidr: string = "10.0.0.0/16"
    }
}

