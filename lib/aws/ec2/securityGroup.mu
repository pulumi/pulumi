// Copyright 2016 Marapongo, Inc. All rights reserved.

module aws/ec2
import aws/cloudformation

// An Amazon EC2 security group.
// @website: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-ec2-security-group.html
service SecurityGroup {
    ctor() {
        cloudformation.ExpandTags(this.properties)
        new cloudformation.Resource {
            resource = "AWS::EC2::SecurityGroup"
            properties = this.properties
        }
    }

    properties: cloudformation.TagArgs {
        // Description of the security group.
        readonly groupDescription: string
        // The VPC in which this security group resides.
        readonly vpc: VPC
        // A list of Amazon EC2 security group egress rules.
        optional securityGroupEgress: SecurityGroupEgressRule[]
        // A list of Amazon EC2 security group ingress rules.
        optional securityGroupIngress: SecurityGroupIngressRule[]
    }
}

// An EC2 Security Group Rule is an embedded property of the SecurityGroup.
schema SecurityGroupRule {
    // An IP protocol name or number.
    ipProtocol: string
    // Specifies a CIDR range.
    optional cidrIp: string
    // The start of port range for the TCP and UDP protocols, or an ICMP type number. An ICMP type number of `-1`
    // indicates a wildcard (i.e., any ICMP type number).
    optional fromPort: number
    // The end of port range for the TCP and UDP protocols, or an ICMP code. An ICMP code of `-1` indicates a wildcard
    // (i.e., any ICMP code).
    optional toPort: number
}

// An EC2 Security Group Egress Rule is an embedded property of the SecurityGroup (different from the resource).
schema SecurityGroupEgressRule: SecurityGroupRule {
    // The AWS service prefix of an Amazon VPC endpoint.
    optional destinationPrefixListId: string;
    // Specifies the destination Amazon VPC security group.
    optional destinationSecurityGroup: SecurityGroup;
}

// An EC2 Security Group Ingress Rule is an embedded property of the SecurityGroup (different from the resource).
schema SecurityGroupIngressRule: SecurityGroupRule {
    // For VPC security groups only. Specifies the ID of the Amazon EC2 Security Group to allow access.
    sourceSecurityGroup: SecurityGroup
    // For non-VPC security groups only. Specifies the name of the Amazon EC2 Security Group to use for access.
    sourceSecurityGroupName: string
    // Specifies the AWS Account ID of the owner of the Amazon EC2 Security Group that is specified in the
    // SourceSecurityGroupName property.
    sourceSecurityGroupOwnerId: string
}

