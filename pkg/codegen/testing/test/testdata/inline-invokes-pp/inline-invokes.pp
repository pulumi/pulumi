resource webSecurityGroup "aws:ec2:SecurityGroup" {
  vpcId = invoke("aws:ec2:getVpc", { default = true }).id
}