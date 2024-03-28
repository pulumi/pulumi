config instanceType string {
	__logicalName = "InstanceType"
	default = "t3.micro"
}

resource webSecGrp "aws:ec2/securityGroup:SecurityGroup" {
	__logicalName = "WebSecGrp"
	ingress = [{
		protocol = "tcp",
		fromPort = 80,
		toPort = 80,
		cidrBlocks = ["0.0.0.0/0"]
	}]

	options {
		version = "4.37.1"
	}
}

resource webServer "aws:ec2/instance:Instance" {
	__logicalName = "WebServer"
	instanceType = instanceType
	ami = invoke("aws:index/getAmi:getAmi", {
		filters = [{
			name = "name",
			values = ["amzn-ami-hvm-*-x86_64-ebs"]
		}],
		owners = ["137112412989"],
		mostRecent = true
	}).id
	userData = join("\n", [
		"#!/bin/bash",
		"echo 'Hello, World from ${webSecGrp.arn}!' > index.html",
		"nohup python -m SimpleHTTPServer 80 &"
	])
	vpcSecurityGroupIds = [webSecGrp.id]

	options {
		version = "4.37.1"
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
