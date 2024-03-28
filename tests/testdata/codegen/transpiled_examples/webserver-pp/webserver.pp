config instanceType string {
	__logicalName = "InstanceType"
	default = "t3.micro"
}

ec2Ami = invoke("aws:index/getAmi:getAmi", {
	filters = [{
		name = "name",
		values = ["amzn-ami-hvm-*-x86_64-ebs"]
	}],
	owners = ["137112412989"],
	mostRecent = true
}).id

resource webSecGrp "aws:ec2/securityGroup:SecurityGroup" {
	__logicalName = "WebSecGrp"
	ingress = [{
		protocol = "tcp",
		fromPort = 80,
		toPort = 80,
		cidrBlocks = ["0.0.0.0/0"]
	}]
}

resource webServer "aws:ec2/instance:Instance" {
	__logicalName = "WebServer"
	instanceType = instanceType
	ami = ec2Ami
	userData = "#!/bin/bash\necho 'Hello, World from ${webSecGrp.arn}!' > index.html\nnohup python -m SimpleHTTPServer 80 &"
	vpcSecurityGroupIds = [webSecGrp.id]
}

resource usEast2Provider "pulumi:providers:aws" {
	__logicalName = "UsEast2Provider"
	region = "us-east-2"
}

resource myBucket "aws:s3/bucket:Bucket" {
	__logicalName = "MyBucket"

	options {
		provider = usEast2Provider
	}
}

output instanceId {
	__logicalName = "InstanceId"
	value = webServer.id
}

output publicIp {
	__logicalName = "PublicIp"
	value = webServer.publicIp
}

output publicHostName {
	__logicalName = "PublicHostName"
	value = webServer.publicDns
}
