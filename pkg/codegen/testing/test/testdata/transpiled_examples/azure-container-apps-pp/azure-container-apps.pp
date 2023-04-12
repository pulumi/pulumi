config sqlAdmin string {
	__logicalName = "sqlAdmin"
	default = "pulumi"
}

config retentionInDays int {
	__logicalName = "retentionInDays"
	default = 30
}

sharedKey = secret(invoke("azure-native:operationalinsights:getSharedKeys", {
	resourceGroupName = resourceGroup.name,
	workspaceName = workspace.name
}).primarySharedKey)
adminRegistryCreds = invoke("azure-native:containerregistry:listRegistryCredentials", {
	resourceGroupName = resourceGroup.name,
	registryName = registry.name
})
adminUsername = adminRegistryCreds.username
adminPasswords = secret(adminRegistryCreds.passwords)

resource resourceGroup "azure-native:resources:ResourceGroup" {
	__logicalName = "resourceGroup"
}

resource workspace "azure-native:operationalinsights:Workspace" {
	__logicalName = "workspace"
	resourceGroupName = resourceGroup.name
	sku = {
		name = "PerGB2018"
	}
	retentionInDays = retentionInDays
}

resource kubeEnv "azure-native:web:KubeEnvironment" {
	__logicalName = "kubeEnv"
	resourceGroupName = resourceGroup.name
	environmentType = "Managed"
	appLogsConfiguration = {
		destination = "log-analytics",
		logAnalyticsConfiguration = {
			customerId = workspace.customerId,
			sharedKey = sharedKey
		}
	}
}

resource registry "azure-native:containerregistry:Registry" {
	__logicalName = "registry"
	resourceGroupName = resourceGroup.name
	sku = {
		name = "Basic"
	}
	adminUserEnabled = true
}

resource provider "pulumi:providers:docker" {
	__logicalName = "provider"
	registryAuth = [{
		address = registry.loginServer,
		username = adminUsername,
		password = adminPasswords[0].value
	}]
}

resource myImage "docker:index/registryImage:RegistryImage" {
	__logicalName = "myImage"
	name = "${registry.loginServer}/node-app:v1.0.0"
	build = {
		context = "${cwd()}/node-app"
	}

	options {
		provider = provider
	}
}

resource containerapp "azure-native:web:ContainerApp" {
	__logicalName = "containerapp"
	resourceGroupName = resourceGroup.name
	kubeEnvironmentId = kubeEnv.id
	configuration = {
		ingress = {
			external = true,
			targetPort = 80
		},
		registries = [{
			server = registry.loginServer,
			username = adminUsername,
			passwordSecretRef = "pwd"
		}],
		secrets = [{
			name = "pwd",
			value = adminPasswords[0].value
		}]
	}
	template = {
		containers = [{
			name = "myapp",
			image = myImage.name
		}]
	}
}

output endpoint {
	__logicalName = "endpoint"
	value = "https://${containerapp.configuration.ingress.fqdn}"
}
