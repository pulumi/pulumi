resource databaseAccount "azure-native:documentdb:DatabaseAccount" {
    accountName = "ddb1"
    apiProperties = {
        serverVersion = "3.2"
    }
    backupPolicy = {
        periodicModeProperties = {
            backupIntervalInMinutes = 240
            backupRetentionIntervalInHours = 8
        }
        type = "Periodic"
    }
    databaseAccountOfferType = "Standard"
    locations = [{
        failoverPriority = 0
        isZoneRedundant = false
        locationName = "sourthcentralus"
    }]
    resourceGroupName = "rg1"
}
