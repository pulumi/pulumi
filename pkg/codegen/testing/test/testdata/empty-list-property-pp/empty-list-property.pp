resource storageAccounts "azure-native:storage:StorageAccount" {
    accountName = "sto4445"
    kind = "BlockBlobStorage"
    location = "eastus"
    resourceGroupName = "res9101"
    sku = { name = "Premium_LRS" }
    networkRuleSet = {
        defaultAction = "Allow"
        ipRules = []
    }
}