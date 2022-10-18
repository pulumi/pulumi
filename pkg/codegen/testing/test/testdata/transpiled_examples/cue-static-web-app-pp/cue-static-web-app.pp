resource rawkodeGroup "azure-native:resources:ResourceGroup" {
	__logicalName = "rawkode-group"
	location = "WestUs"
}

resource rawkodeStorage "azure-native:storage:StorageAccount" {
	__logicalName = "rawkode-storage"
	resourceGroupName = rawkodeGroup.name
	kind = "StorageV2"
	sku = {
		name = "Standard_LRS"
	}
}

resource rawkodeWebsite "azure-native:storage:StorageAccountStaticWebsite" {
	__logicalName = "rawkode-website"
	resourceGroupName = rawkodeGroup.name
	accountName = rawkodeStorage.name
	indexDocument = "index.html"
	error404Document = "404.html"
}

resource rawkodeIndexHtml "azure-native:storage:Blob" {
	__logicalName = "rawkode-index.html"
	resourceGroupName = rawkodeGroup.name
	accountName = rawkodeStorage.name
	containerName = rawkodeWebsite.containerName
	contentType = "text/html"
	type = "Block"
	source = fileAsset("./website/index.html")
}

resource stack72Group "azure-native:resources:ResourceGroup" {
	__logicalName = "stack72-group"
	location = "WestUs"
}

resource stack72Storage "azure-native:storage:StorageAccount" {
	__logicalName = "stack72-storage"
	resourceGroupName = stack72Group.name
	kind = "StorageV2"
	sku = {
		name = "Standard_LRS"
	}
}

resource stack72Website "azure-native:storage:StorageAccountStaticWebsite" {
	__logicalName = "stack72-website"
	resourceGroupName = stack72Group.name
	accountName = stack72Storage.name
	indexDocument = "index.html"
	error404Document = "404.html"
}

resource stack72IndexHtml "azure-native:storage:Blob" {
	__logicalName = "stack72-index.html"
	resourceGroupName = stack72Group.name
	accountName = stack72Storage.name
	containerName = stack72Website.containerName
	contentType = "text/html"
	type = "Block"
	source = fileAsset("./website/index.html")
}
