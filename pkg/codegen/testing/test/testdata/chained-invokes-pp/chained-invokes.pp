vpcId = invoke("aws:ec2/getVpc:getVpc", {
	default = true
}).id

subnetIds = invoke("aws:ec2/getSubnetIds:getSubnetIds", {
	vpcId = vpcId
}).ids
