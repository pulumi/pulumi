import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";
import * as std from "@pulumi/std";

function notImplemented(message: string) {
    throw new Error(message);
}

const config = new pulumi.Config();
// Controls if VPC should be created (it affects almost all resources)
const createVpc = config.getBoolean("createVpc") || true;
// Name to be used on all the resources as identifier
const name = config.get("name") || "";
// (Optional) The IPv4 CIDR block for the VPC. CIDR can be explicitly set or it can be derived from IPAM using `ipv4_netmask_length` & `ipv4_ipam_pool_id`
const cidr = config.get("cidr") || "10.0.0.0/16";
// A list of availability zones names or ids in the region
const azs = config.getObject("azs") || [];
// A list of public subnets inside the VPC
const publicSubnets = config.getObject("publicSubnets") || [];
// Assigns IPv6 public subnet id based on the Amazon provided /56 prefix base 10 integer (0-256). Must be of equal length to the corresponding IPv4 subnet list
const publicSubnetIpv6Prefixes = config.getObject("publicSubnetIpv6Prefixes") || [];
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
const lenPublicSubnets = std.maxOutput({
    input: [
        1,
        2,
        3,
    ],
}).apply(invoke => invoke.result);
const _this = new aws.ec2.Vpc("this", {});
const createPublicSubnets = true;
const _public: aws.ec2.Subnet[] = [];
(createPublicSubnets && (!oneNatGatewayPerAz || lenPublicSubnets >= azs.length) ? lenPublicSubnets : 0).apply(rangeBody => {
    for (const range = {value: 0}; range.value < rangeBody; range.value++) {
        _public.push(new aws.ec2.Subnet(`public-${range.value}`, {
            assignIpv6AddressOnCreation: enableIpv6 && publicSubnetIpv6Native ? true : publicSubnetAssignIpv6AddressOnCreation,
            availabilityZone: notImplemented("regexall(\"^[a-z]{2}-\",element(var.azs,count.index))").length > 0 ? notImplemented("element(var.azs,count.index)") : undefined,
            availabilityZoneId: notImplemented("regexall(\"^[a-z]{2}-\",element(var.azs,count.index))").length == 0 ? notImplemented("element(var.azs,count.index)") : undefined,
            cidrBlock: publicSubnetIpv6Native ? undefined : notImplemented("element(concat(var.public_subnets,[\"\"]),count.index)"),
            enableDns64: enableIpv6 && publicSubnetEnableDns64,
            enableResourceNameDnsAaaaRecordOnLaunch: enableIpv6 && publicSubnetEnableResourceNameDnsAaaaRecordOnLaunch,
            enableResourceNameDnsARecordOnLaunch: !publicSubnetIpv6Native && publicSubnetEnableResourceNameDnsARecordOnLaunch,
            ipv6CidrBlock: enableIpv6 && publicSubnetIpv6Prefixes.length > 0 ? std.cidrsubnetOutput({
                input: _this.ipv6CidrBlock,
                newbits: 8,
                netnum: publicSubnetIpv6Prefixes[range.value],
            }).apply(invoke => invoke.result) : undefined,
            ipv6Native: enableIpv6 && publicSubnetIpv6Native,
            vpcId: _this.id,
        }));
    }
});
