resource cluster "aws:ecs/cluster:Cluster" {
	__logicalName = "cluster"
}

resource loadBalancer "awsx:lb:ApplicationLoadBalancer" {
	__logicalName = "loadBalancer"
}

resource nginx "awsx:ecs:FargateService" {
	__logicalName = "nginx"
	cluster = cluster.arn
	taskDefinitionArgs = {
		container = {
			image = "nginx:latest",
			cpu = 512,
			memory = 128,
			portMappings = [{
				containerPort = 80,
				targetGroup = loadBalancer.defaultTargetGroup
			}]
		}
	}
}

output url {
	__logicalName = "url"
	value = loadBalancer.loadBalancer.dnsName
}
