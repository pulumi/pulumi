import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";
import * as std from "@pulumi/std";

export = async () => {
    const config = new pulumi.Config();
    // A list of availability zones names or ids in the region
    const azs = config.getObject<Array<string>>("azs") || [];
    // Assigns IPv6 public subnet id based on the Amazon provided /56 prefix base 10 integer (0-256). Must be of equal length to the corresponding IPv4 subnet list
    const publicSubnetIpv6Prefixes = config.getObject<Array<string>>("publicSubnetIpv6Prefixes") || [];
    // Should be true if you want only one NAT Gateway per availability zone. Requires `var.azs` to be set, and the number of `public_subnets` created to be greater than or equal to the number of availability zones specified in `var.azs`
    const oneNatGatewayPerAz = config.getBoolean("oneNatGatewayPerAz") || false;
    // Requests an Amazon-provided IPv6 CIDR block with a /56 prefix length for the VPC. You cannot specify the range of IP addresses, or the size of the CIDR block
    const enableIpv6 = config.getBoolean("enableIpv6") || false;
    // Indicates whether to create an IPv6-only subnet. Default: `false`
    const publicSubnetIpv6Native = config.getBoolean("publicSubnetIpv6Native") || false;
    // Indicates whether DNS queries made to the Amazon-provided DNS Resolver in this subnet should return synthetic IPv6 addresses for IPv4-only destinations. Default: `true`
    const publicSubnetEnableDns64 = config.getBoolean("publicSubnetEnableDns64") || true;
    // Specify true to indicate that network interfaces created in the specified subnet should be assigned an IPv6 address. Default is `false`
    const publicSubnetAssignIpv6AddressOnCreation = config.getBoolean("publicSubnetAssignIpv6AddressOnCreation") || false;
    // Indicates whether to respond to DNS queries for instance hostnames with DNS AAAA records. Default: `true`
    const publicSubnetEnableResourceNameDnsAaaaRecordOnLaunch = config.getBoolean("publicSubnetEnableResourceNameDnsAaaaRecordOnLaunch") || true;
    // Indicates whether to respond to DNS queries for instance hostnames with DNS A records. Default: `false`
    const publicSubnetEnableResourceNameDnsARecordOnLaunch = config.getBoolean("publicSubnetEnableResourceNameDnsARecordOnLaunch") || false;
    const lenPublicSubnets = (await std.max({
        input: [
            1,
            2,
            3,
        ],
    })).result;
    const currentVpc = new aws.ec2.Vpc("currentVpc", {});
    const createPublicSubnets = true;
    const publicSubnet: aws.ec2.Subnet[] = [];
    for (const range = {value: 0}; range.value < (createPublicSubnets && (!oneNatGatewayPerAz || lenPublicSubnets >= azs.length) ? lenPublicSubnets : 0); range.value++) {
        publicSubnet.push(new aws.ec2.Subnet(`publicSubnet-${range.value}`, {
            assignIpv6AddressOnCreation: enableIpv6 && publicSubnetIpv6Native ? true : publicSubnetAssignIpv6AddressOnCreation,
            enableDns64: enableIpv6 && publicSubnetEnableDns64,
            enableResourceNameDnsAaaaRecordOnLaunch: enableIpv6 && publicSubnetEnableResourceNameDnsAaaaRecordOnLaunch,
            enableResourceNameDnsARecordOnLaunch: !publicSubnetIpv6Native && publicSubnetEnableResourceNameDnsARecordOnLaunch,
            ipv6CidrBlock: enableIpv6 && publicSubnetIpv6Prefixes.length > 0 ? currentVpc.ipv6CidrBlock.apply(ipv6CidrBlock => std.cidrsubnetOutput({
                input: ipv6CidrBlock,
                newbits: 8,
                netnum: publicSubnetIpv6Prefixes[range.value],
            })).apply(invoke => invoke.result) : null,
            ipv6Native: enableIpv6 && publicSubnetIpv6Native,
            vpcId: currentVpc.id,
        }));
    }
}
