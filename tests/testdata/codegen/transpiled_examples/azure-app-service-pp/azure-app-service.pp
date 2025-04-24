config sqlAdmin string {
	__logicalName = "sqlAdmin"
	default = "pulumi"
}

blobAccessToken = secret(invoke("azure-native:storage:listStorageAccountServiceSAS", {
	accountName = sa.name,
	protocols = "https",
	sharedAccessStartTime = "2022-01-01",
	sharedAccessExpiryTime = "2030-01-01",
	resource = "c",
	resourceGroupName = appservicegroup.name,
	permissions = "r",
	canonicalizedResource = "/blob/${sa.name}/${container.name}",
	contentType = "application/json",
	cacheControl = "max-age=5",
	contentDisposition = "inline",
	contentEncoding = "deflate"
}).serviceSasToken)

resource appservicegroup "azure-native:resources:ResourceGroup" {
	__logicalName = "appservicegroup"
}

resource sa "azure-native:storage:StorageAccount" {
	__logicalName = "sa"
	resourceGroupName = appservicegroup.name
	kind = "StorageV2"
	sku = {
		name = "Standard_LRS"
	}
}

resource appserviceplan "azure-native:web:AppServicePlan" {
	__logicalName = "appserviceplan"
	resourceGroupName = appservicegroup.name
	kind = "App"
	sku = {
		name = "B1",
		tier = "Basic"
	}
}

resource container "azure-native:storage:BlobContainer" {
	__logicalName = "container"
	resourceGroupName = appservicegroup.name
	accountName = sa.name
	publicAccess = "None"
}

resource blob "azure-native:storage:Blob" {
	__logicalName = "blob"
	resourceGroupName = appservicegroup.name
	accountName = sa.name
	containerName = container.name
	type = "Block"
	source = fileArchive("./www")
}

resource appInsights "azure-native:insights:Component" {
	__logicalName = "appInsights"
	resourceGroupName = appservicegroup.name
	applicationType = "web"
	kind = "web"
}

resource sqlPassword "random:index/randomPassword:RandomPassword" {
	__logicalName = "sqlPassword"
	length = 16
	special = true
}

resource sqlServer "azure-native:sql:Server" {
	__logicalName = "sqlServer"
	resourceGroupName = appservicegroup.name
	administratorLogin = sqlAdmin
	administratorLoginPassword = sqlPassword.result
	version = "12.0"
}

resource db "azure-native:sql:Database" {
	__logicalName = "db"
	resourceGroupName = appservicegroup.name
	serverName = sqlServer.name
	sku = {
		name = "S0"
	}
}

resource app "azure-native:web:WebApp" {
	__logicalName = "app"
	resourceGroupName = appservicegroup.name
	serverFarmId = appserviceplan.id
	siteConfig = {
		appSettings = [
			{
				name = "WEBSITE_RUN_FROM_PACKAGE",
				value = "https://${sa.name}.blob.core.windows.net/${container.name}/${blob.name}?${blobAccessToken}"
			},
			{
				name = "APPINSIGHTS_INSTRUMENTATIONKEY",
				value = appInsights.instrumentationKey
			},
			{
				name = "APPLICATIONINSIGHTS_CONNECTION_STRING",
				value = "InstrumentationKey=${appInsights.instrumentationKey}"
			},
			{
				name = "ApplicationInsightsAgent_EXTENSION_VERSION",
				value = "~2"
			}
		],
		connectionStrings = [{
			name = "db",
			type = "SQLAzure",
			connectionString = "Server= tcp:${sqlServer.name}.database.windows.net;initial catalog=${db.name};userID=${sqlAdmin};password=${sqlPassword.result};Min Pool Size=0;Max Pool Size=30;Persist Security Info=true;"
		}]
	}
}

output endpoint {
	__logicalName = "endpoint"
	value = app.defaultHostName
}
