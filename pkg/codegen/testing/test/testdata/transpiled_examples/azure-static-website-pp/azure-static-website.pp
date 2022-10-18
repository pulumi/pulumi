resource staticsitegroup "azure-native:resources:ResourceGroup" {
	__logicalName = "staticsitegroup"
}

resource storageaccount "azure-native:storage:StorageAccount" {
	__logicalName = "storageaccount"
	resourceGroupName = staticsitegroup.name
	kind = "StorageV2"
	sku = {
		name = "Standard_LRS"
	}
}

resource staticwebsite "azure-native:storage:StorageAccountStaticWebsite" {
	__logicalName = "staticwebsite"
	resourceGroupName = staticsitegroup.name
	accountName = storageaccount.name
	indexDocument = "index.html"
	error404Document = "404.html"
}

resource indexHtml "azure-native:storage:Blob" {
	__logicalName = "index.html"
	resourceGroupName = staticsitegroup.name
	accountName = storageaccount.name
	containerName = staticwebsite.containerName
	contentType = "text/html"
	type = "Block"
	source = fileAsset("./www/index.html")
}

resource faviconPng "azure-native:storage:Blob" {
	__logicalName = "favicon.png"
	resourceGroupName = staticsitegroup.name
	accountName = storageaccount.name
	containerName = staticwebsite.containerName
	contentType = "image/png"
	type = "Block"
	source = fileAsset("./www/favicon.png")
}

resource "404Html" "azure-native:storage:Blob" {
	__logicalName = "404.html"
	resourceGroupName = staticsitegroup.name
	accountName = storageaccount.name
	containerName = staticwebsite.containerName
	contentType = "text/html"
	type = "Block"
	source = fileAsset("./www/404.html")
}

output endpoint {
	__logicalName = "endpoint"
	value = storageaccount.primaryEndpoints.web
}
