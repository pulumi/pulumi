someString = "foobar"

typeVar = "Block"

resource staticwebsite "azure-native:storage:StorageAccountStaticWebsite" {
	resourceGroupName = someString
	accountName = someString
}

// Safe enum
resource faviconpng "azure-native:storage:Blob" {
	resourceGroupName = someString
	accountName = someString
	containerName = someString
	type = "Block"
}

// Output umsafe enum
resource _404html "azure-native:storage:Blob" {
	resourceGroupName = someString
	accountName = someString
	containerName = someString
	type = staticwebsite.indexDocument
}

// Unsafe enum
resource another "azure-native:storage:Blob" {
	resourceGroupName = someString
	accountName = someString
	containerName = someString
	type = typeVar
}
