resource "vpc" "aws:ec2/vpc:Vpc" {
  cidrBlock          = "10.0.0.0/16"
  enableDnsHostnames = true
  enableDnsSupport   = true
  tags = {
    Name = "Example VPC"
  }
}

resource "gateway" "aws:ec2/internetGateway:InternetGateway" {
  options {
    range = vpc.tags
  }
  vpcId = "${vpc.id}"
  tags = {
    Name = "${range.key} ${range.value}"
  }
}
