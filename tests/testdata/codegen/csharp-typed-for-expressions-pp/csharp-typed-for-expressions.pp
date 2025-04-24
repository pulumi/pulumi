config egress "list(object({FromPort=int, ToPort=int}))" {

}

config tags "map(string)" {
    default = {}
}

defaultVpc = invoke("aws:ec2:getVpc", {
	default = true
})

// Create a security group that permits HTTP ingress and unrestricted egress.
resource webSecurityGroup "aws:ec2:SecurityGroup" {
	vpcId = defaultVpc.id
	egress = [for entry in entries(egress): {
		protocol = "-1"
		fromPort = entry.value.FromPort
		toPort = entry.value.ToPort
		cidrBlocks = ["0.0.0.0/0"]
	}]

	tags = { for k, v in tags: k => v }
}