config storageAccountNameParam string {
    description = "The name of the storage account"
}

config resourceGroupNameParam string {
    description = "The name of the resource group"
}

resourceGroupVar = invoke("azure:core/getResourceGroup:getResourceGroup", {
	name = resourceGroupNameParam
})

config locationParam string {
	default = resourceGroupVar.location
}

config storageAccountTierParam string {
    default = "Standard"
}

config storageAccountTypeReplicationParam string {
    default = "LRS"
}

resource storageAccountResource "azure:storage/account:Account" {
	name = storageAccountNameParam
	accountKind = "StorageV2"
	location = locationParam
	resourceGroupName = resourceGroupNameParam
	accountTier = storageAccountTierParam
	accountReplicationType = storageAccountTypeReplicationParam
}

output storageAccountNameOut {
	value = storageAccountResource.name
}
