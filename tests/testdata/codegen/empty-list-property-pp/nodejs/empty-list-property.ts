import * as pulumi from "@pulumi/pulumi";
import * as azure_native from "@pulumi/azure-native";

const storageAccounts = new azure_native.storage.StorageAccount("storageAccounts", {
    accountName: "sto4445",
    kind: azure_native.storage.Kind.BlockBlobStorage,
    location: "eastus",
    resourceGroupName: "res9101",
    sku: {
        name: azure_native.storage.SkuName.Premium_LRS,
    },
    networkRuleSet: {
        defaultAction: azure_native.storage.DefaultAction.Allow,
        ipRules: [],
    },
});
