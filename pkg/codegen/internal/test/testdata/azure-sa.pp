config resourceGroupNameParam string {
}

resourceGroupVar = invoke("azure:core/getResourceGroup:getResourceGroup", {
	name = resourceGroupNameParam
})

config locationParam string {
	default = resourceGroupVar.location
}

