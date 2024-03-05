resource argocd_serverDeployment "kubernetes:apps/v1:Deployment" {
	apiVersion = "apps/v1"
	kind = "Deployment"
	metadata = {
		name = "argocd-server"
	}
	spec = {
		selector = {
			matchLabels = {
				app = "server"
			}
		}
		replicas = 1
		template = {
			metadata = {
				labels = {
					app = "server"
				}
			}
			spec = {
				containers = [
					{
						name = "nginx"
						image = "nginx"
						readinessProbe = {
							httpGet = {
								port = 8080
							}
						}
					}
				]
			}
		}
	}
}
