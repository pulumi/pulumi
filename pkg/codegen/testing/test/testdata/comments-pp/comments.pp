// Test comments for a resource
resource securityGroup "aws:ec2:SecurityGroup" {
    // Test comments for a property
	ingress = [{
        // Test comments for a map entry
		protocol = "tcp"
		fromPort = 0
		toPort = 0
		cidrBlocks = [
            // Test comments for an array entry
            "0.0.0.0/0"
        ]
	}]
}

// Test comments for a local
ami = invoke("aws:index:getAmi", {
	filters = [{
		name = "name" // Test trailing map item comment
		values = ["amzn-ami-hvm-*-x86_64-ebs"]
	}]
	owners = [
        "137112412989" // Test trailing array item comment
    ]
	mostRecent = true // Test trailing property comment
})

// Test comments for another resource
resource server "aws:ec2:Instance" {
	tags = {
		Name = "web-server-www"
	} // Test trailing map comment
	instanceType = "t2.micro"
	securityGroups = [securityGroup.name] // Test trailing array comment
	ami = ami.id
	userData = <<-EOF
		#!/bin/bash
		echo "Hello, World!" > index.html
		nohup python -m SimpleHTTPServer 80 &
	EOF
	// Final trailing resource comment
}

// Test output comment
output secGroupName {
    // Test output value comment
    value = securityGroup.name
}
