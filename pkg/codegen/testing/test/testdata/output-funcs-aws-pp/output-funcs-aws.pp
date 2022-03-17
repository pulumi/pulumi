resource aws_vpc "aws:ec2/vpc:Vpc" {
  cidrBlock       = "10.0.0.0/16"
  instanceTenancy = "default"
}

resource privateS3VpcEndpoint "aws:ec2/vpcEndpoint:VpcEndpoint" {
  vpcId       = aws_vpc.id
  serviceName = "com.amazonaws.us-west-2.s3"
}

privateS3PrefixList = invoke("aws:ec2:getPrefixList", {
  prefixListId = privateS3VpcEndpoint.prefixListId
})

resource bar "aws:ec2/networkAcl:NetworkAcl" {
  vpcId = aws_vpc.id
}

resource privateS3NetworkAclRule "aws:ec2/networkAclRule:NetworkAclRule" {
  networkAclId = bar.id
  ruleNumber   = 200
  egress       = false
  protocol     = "tcp"
  ruleAction   = "allow"
  cidrBlock    = privateS3PrefixList.cidrBlocks[0]
  fromPort     = 443
  toPort       = 443
}

# A contrived example to test that helper nested records ( `filters`
# below) generate correctly when using output-versioned function
# invoke forms.
amis = invoke("aws:ec2:getAmiIds", {
  owners = [bar.id]
  filters = [{name=bar.id, values=["pulumi*"]}]
})
