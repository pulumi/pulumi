resource webServer "aws:ec2/instance:Instance" {
	ami = invoke("aws:index/getAmi:getAmi", {
		"filters" = [{
			"name" = "name",
			"values" = ["amzn-ami-hvm-*-x86_64-ebs"]
		}],
		"owners" = ["137112412989"],
		"mostRecent" = true
	}).id
}
