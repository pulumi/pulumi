// Copyright 2016 Marapongo, Inc. All rights reserved.

module aws/ec2
import aws/cloudformation

// Adds an ingress rule to an Amazon VPC security group.
// @website: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ec2-security-group-ingress.html
service SecurityGroupIngress {
    new() {
        cf := new cloudformation.Resource {
            resource: "AWS::EC2::SecurityGroupIngress"
            properties: this.properties
        }
    }

    properties {
        // IP protocol name or number.
        readonly ipProtocol: string
        // An IPv4 CIDR range.
        optional readonly cidrIp: string
        // An IPv6 CIDR range.
        optional readonly cidrIpv6: string
        // Start of port range for the TCP and UDP protocols, or an ICMP type number. If you specify `icmp` for the
        // `ipProtocol` property, you can specify `-1` as a wildcard (i.e., any ICMP type number).
        optional readonly fromPort: number
        // The Amazon VPC security group to modify.
        optional readonly group: SecurityGroup
        // Name of the Amazon EC2 security group (non-VPC security group) to modify.
        optional readonly groupName: string
        // Specifies the ID of the source security group or uses the Ref intrinsic function to refer to the logical ID of a
        // security group defined in the same template.
        optional readonly sourceSecurityGroup: SecurityGroup
        // Specifies the name of the Amazon EC2 security group (non-VPC security group) to allow access or uses the Ref
        // intrinsic function to refer to the logical name of a security group defined in the same template. For instances
        // in a VPC, specify the SourceSecurityGroupId property.
        optional readonly sourceSecurityGroupName: string
        // Specifies the AWS Account ID of the owner of the Amazon EC2 security group specified in the
        // SourceSecurityGroupName property.
        optional readonly sourceSecurityGroupOwnerId: string
        // End of port range for the TCP and UDP protocols, or an ICMP code. If you specify `icmp` for the `ipProtocol`
        // property, you can specify `-1` as a wildcard (i.e., any ICMP code).
        optional readonly toPort: number
    }
}

