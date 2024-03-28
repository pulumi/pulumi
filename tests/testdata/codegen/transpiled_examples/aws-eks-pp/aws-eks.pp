vpcId = invoke("aws:ec2/getVpc:getVpc", {
	default = true
}).id
subnetIds = invoke("aws:ec2/getSubnetIds:getSubnetIds", {
	vpcId = vpcId
}).ids

resource cluster "eks:index:Cluster" {
	__logicalName = "cluster"
	vpcId = vpcId
	subnetIds = subnetIds
	instanceType = "t2.medium"
	desiredCapacity = 2
	minSize = 1
	maxSize = 2
}

output kubeconfig {
	__logicalName = "kubeconfig"
	value = cluster.kubeconfig
}
