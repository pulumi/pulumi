import * as pulumi from "@pulumi/pulumi";
import * as azure_native from "@pulumi/azure-native";

const databaseAccount = new azure_native.documentdb.DatabaseAccount("databaseAccount", {
    accountName: "ddb1",
    apiProperties: {
        serverVersion: "3.2",
    },
    backupPolicy: {
        periodicModeProperties: {
            backupIntervalInMinutes: 240,
            backupRetentionIntervalInHours: 8,
        },
        type: "Periodic",
    },
    databaseAccountOfferType: azure_native.documentdb.DatabaseAccountOfferType.Standard,
    locations: [{
        failoverPriority: 0,
        isZoneRedundant: false,
        locationName: "sourthcentralus",
    }],
    resourceGroupName: "rg1",
});
