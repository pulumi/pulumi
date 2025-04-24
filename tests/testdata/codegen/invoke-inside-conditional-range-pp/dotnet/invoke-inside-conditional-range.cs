using System.Collections.Generic;
using System.Linq;
using System.Threading.Tasks;
using Pulumi;
using Aws = Pulumi.Aws;
using Std = Pulumi.Std;

return await Deployment.RunAsync(async() => 
{
    var config = new Config();
    // A list of availability zones names or ids in the region
    var azs = config.GetObject<string[]>("azs") ?? new[] {};
    // Assigns IPv6 public subnet id based on the Amazon provided /56 prefix base 10 integer (0-256). Must be of equal length to the corresponding IPv4 subnet list
    var publicSubnetIpv6Prefixes = config.GetObject<string[]>("publicSubnetIpv6Prefixes") ?? new[] {};
    // Should be true if you want only one NAT Gateway per availability zone. Requires `var.azs` to be set, and the number of `public_subnets` created to be greater than or equal to the number of availability zones specified in `var.azs`
    var oneNatGatewayPerAz = config.GetBoolean("oneNatGatewayPerAz") ?? false;
    // Requests an Amazon-provided IPv6 CIDR block with a /56 prefix length for the VPC. You cannot specify the range of IP addresses, or the size of the CIDR block
    var enableIpv6 = config.GetBoolean("enableIpv6") ?? false;
    // Indicates whether to create an IPv6-only subnet. Default: `false`
    var publicSubnetIpv6Native = config.GetBoolean("publicSubnetIpv6Native") ?? false;
    // Indicates whether DNS queries made to the Amazon-provided DNS Resolver in this subnet should return synthetic IPv6 addresses for IPv4-only destinations. Default: `true`
    var publicSubnetEnableDns64 = config.GetBoolean("publicSubnetEnableDns64") ?? true;
    // Specify true to indicate that network interfaces created in the specified subnet should be assigned an IPv6 address. Default is `false`
    var publicSubnetAssignIpv6AddressOnCreation = config.GetBoolean("publicSubnetAssignIpv6AddressOnCreation") ?? false;
    // Indicates whether to respond to DNS queries for instance hostnames with DNS AAAA records. Default: `true`
    var publicSubnetEnableResourceNameDnsAaaaRecordOnLaunch = config.GetBoolean("publicSubnetEnableResourceNameDnsAaaaRecordOnLaunch") ?? true;
    // Indicates whether to respond to DNS queries for instance hostnames with DNS A records. Default: `false`
    var publicSubnetEnableResourceNameDnsARecordOnLaunch = config.GetBoolean("publicSubnetEnableResourceNameDnsARecordOnLaunch") ?? false;
    var lenPublicSubnets = (await Std.Max.InvokeAsync(new()
    {
        Input = new[]
        {
            1,
            2,
            3,
        },
    })).Result;

    var currentVpc = new Aws.Ec2.Vpc("currentVpc");

    var createPublicSubnets = true;

    var publicSubnet = new List<Aws.Ec2.Subnet>();
    for (var rangeIndex = 0; rangeIndex < createPublicSubnets && (!oneNatGatewayPerAz || lenPublicSubnets >= azs.Length) ? lenPublicSubnets : 0; rangeIndex++)
    {
        var range = new { Value = rangeIndex };
        publicSubnet.Add(new Aws.Ec2.Subnet($"publicSubnet-{range.Value}", new()
        {
            AssignIpv6AddressOnCreation = enableIpv6 && publicSubnetIpv6Native ? true : publicSubnetAssignIpv6AddressOnCreation,
            EnableDns64 = enableIpv6 && publicSubnetEnableDns64,
            EnableResourceNameDnsAaaaRecordOnLaunch = enableIpv6 && publicSubnetEnableResourceNameDnsAaaaRecordOnLaunch,
            EnableResourceNameDnsARecordOnLaunch = !publicSubnetIpv6Native && publicSubnetEnableResourceNameDnsARecordOnLaunch,
            Ipv6CidrBlock = enableIpv6 && publicSubnetIpv6Prefixes.Length > 0 ? currentVpc.Ipv6CidrBlock.Apply(ipv6CidrBlock => Std.Cidrsubnet.Invoke(new()
            {
                Input = ipv6CidrBlock,
                Newbits = 8,
                Netnum = publicSubnetIpv6Prefixes[range.Value],
            })).Apply(invoke => invoke.Result) : null,
            Ipv6Native = enableIpv6 && publicSubnetIpv6Native,
            VpcId = currentVpc.Id,
        }));
    }
});

