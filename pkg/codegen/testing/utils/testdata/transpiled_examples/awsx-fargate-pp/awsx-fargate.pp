resource cluster "aws:ecs/cluster:Cluster" {
	__logicalName = "cluster"
}

resource lb "awsx:lb:ApplicationLoadBalancer" {
	__logicalName = "lb"
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
				targetGroup = lb.defaultTargetGroup
			}]
		}
	}
}

output url {
	__logicalName = "url"
	value = lb.loadBalancer.dnsName
}
