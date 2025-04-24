config "localGatewayVirtualInterfaceGroupId" "string" {
}

rts = invoke("aws:ec2/getLocalGatewayRouteTables:getLocalGatewayRouteTables", {
  filters = [{
    name   = "tag:kubernetes.io/kops/role"
    values = ["private*"]
  }]
})

resource "routes" "aws:ec2/localGatewayRoute:LocalGatewayRoute" {
  options {
    range = length(rts.ids)
  }
  destinationCidrBlock                = "10.0.1.0/22"
  localGatewayRouteTableId            = rts.ids[range.value]
  localGatewayVirtualInterfaceGroupId = localGatewayVirtualInterfaceGroupId
}
