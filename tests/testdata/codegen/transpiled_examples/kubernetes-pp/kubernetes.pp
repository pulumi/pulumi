config hostname string {
	__logicalName = "hostname"
	default = "example.com"
}

resource nginxDemo "kubernetes:core/v1:Namespace" {
	__logicalName = "nginx-demo"
}

resource app "kubernetes:apps/v1:Deployment" {
	__logicalName = "app"
	metadata = {
		namespace = nginxDemo.metadata.name
	}
	spec = {
		selector = {
			matchLabels = {
				"app.kubernetes.io/name" = "nginx-demo"
			}
		},
		replicas = 1,
		template = {
			metadata = {
				labels = {
					"app.kubernetes.io/name" = "nginx-demo"
				}
			},
			spec = {
				containers = [{
					name = "app",
					image = "nginx:1.15-alpine"
				}]
			}
		}
	}
}

resource service "kubernetes:core/v1:Service" {
	__logicalName = "service"
	metadata = {
		namespace = nginxDemo.metadata.name,
		labels = {
			"app.kubernetes.io/name" = "nginx-demo"
		}
	}
	spec = {
		type = "ClusterIP",
		ports = [{
			port = 80,
			targetPort = 80,
			protocol = "TCP"
		}],
		selector = {
			"app.kubernetes.io/name" = "nginx-demo"
		}
	}
}

resource ingress "kubernetes:networking.k8s.io/v1:Ingress" {
	__logicalName = "ingress"
	metadata = {
		namespace = nginxDemo.metadata.name
	}
	spec = {
		rules = [{
			host = hostname,
			http = {
				paths = [{
					path = "/",
					pathType = "Prefix",
					backend = {
						service = {
							name = service.metadata.name,
							port = {
								number = 80
							}
						}
					}
				}]
			}
		}]
	}
}
