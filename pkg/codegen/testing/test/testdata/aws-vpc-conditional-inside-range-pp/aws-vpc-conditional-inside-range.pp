config "createVpc" "bool" {
  default     = true
  description = "Controls if VPC should be created (it affects almost all resources)"
}
config "name" "string" {
  default     = ""
  description = "Name to be used on all the resources as identifier"
}
config "cidr" "string" {
  default     = "10.0.0.0/16"
  description = "(Optional) The IPv4 CIDR block for the VPC. CIDR can be explicitly set or it can be derived from IPAM using `ipv4_netmask_length` & `ipv4_ipam_pool_id`"
}

config "azs" "list(string)" {
  default     = []
  description = "A list of availability zones names or ids in the region"
}

config "publicSubnets" "list(string)" {
  default     = []
  description = "A list of public subnets inside the VPC"
}

config "publicSubnetIpv6Prefixes" "list(string)" {
  default     = []
  description = "Assigns IPv6 public subnet id based on the Amazon provided /56 prefix base 10 integer (0-256). Must be of equal length to the corresponding IPv4 subnet list"
}
config "oneNatGatewayPerAz" "bool" {
  default     = false
  description = "Should be true if you want only one NAT Gateway per availability zone. Requires `var.azs` to be set, and the number of `public_subnets` created to be greater than or equal to the number of availability zones specified in `var.azs`"
}

config "enableIpv6" "bool" {
  default     = false
  description = "Requests an Amazon-provided IPv6 CIDR block with a /56 prefix length for the VPC. You cannot specify the range of IP addresses, or the size of the CIDR block"
}

config "publicSubnetIpv6Native" "bool" {
  default     = false
  description = "Indicates whether to create an IPv6-only subnet. Default: `false`"
}

config "publicSubnetEnableDns64" "bool" {
  default     = true
  description = "Indicates whether DNS queries made to the Amazon-provided DNS Resolver in this subnet should return synthetic IPv6 addresses for IPv4-only destinations. Default: `true`"
}

config "publicSubnetAssignIpv6AddressOnCreation" "bool" {
  default     = false
  description = "Specify true to indicate that network interfaces created in the specified subnet should be assigned an IPv6 address. Default is `false`"
}

config "publicSubnetEnableResourceNameDnsAaaaRecordOnLaunch" "bool" {
  default     = true
  description = "Indicates whether to respond to DNS queries for instance hostnames with DNS AAAA records. Default: `true`"
}

config "publicSubnetEnableResourceNameDnsARecordOnLaunch" "bool" {
  default     = false
  description = "Indicates whether to respond to DNS queries for instance hostnames with DNS A records. Default: `false`"
}

lenPublicSubnets = invoke("std:index:max", {
  input = [1, 2, 3]
}).result

resource "this" "aws:ec2/vpc:Vpc" {}

createPublicSubnets = true
resource "public" "aws:ec2/subnet:Subnet" {
  options {
    range = createPublicSubnets && (!oneNatGatewayPerAz || lenPublicSubnets >= length(azs)) ? lenPublicSubnets : 0
  }
  assignIpv6AddressOnCreation             = enableIpv6 && publicSubnetIpv6Native ? true : publicSubnetAssignIpv6AddressOnCreation
  availabilityZone                        = length(notImplemented("regexall(\"^[a-z]{2}-\",element(var.azs,count.index))")) > 0 ? notImplemented("element(var.azs,count.index)") : null
  availabilityZoneId                      = length(notImplemented("regexall(\"^[a-z]{2}-\",element(var.azs,count.index))")) == 0 ? notImplemented("element(var.azs,count.index)") : null
  cidrBlock                               = publicSubnetIpv6Native ? null : notImplemented("element(concat(var.public_subnets,[\"\"]),count.index)")
  enableDns64                             = enableIpv6 && publicSubnetEnableDns64
  enableResourceNameDnsAaaaRecordOnLaunch = enableIpv6 && publicSubnetEnableResourceNameDnsAaaaRecordOnLaunch
  enableResourceNameDnsARecordOnLaunch    = !publicSubnetIpv6Native && publicSubnetEnableResourceNameDnsARecordOnLaunch
  ipv6CidrBlock = enableIpv6 && length(publicSubnetIpv6Prefixes) > 0 ? invoke("std:index:cidrsubnet", {
    input   = this.ipv6CidrBlock
    newbits = 8
    netnum  = publicSubnetIpv6Prefixes[range.value]
  }).result : null
  ipv6Native                     = enableIpv6 && publicSubnetIpv6Native
  vpcId                          = this.id
}