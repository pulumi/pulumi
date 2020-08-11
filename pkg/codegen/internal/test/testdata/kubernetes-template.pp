resource argocd_serverDeployment "kubernetes:apps/v1:Deployment" {
	apiVersion = "apps/v1"
	kind = "Deployment"
	metadata = {
		name = "argocd-server"
	}
	spec = {
		template = {
			spec = {
				containers = [
					{
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