import * as pulumi from "@pulumi/pulumi";
import * as azure_native from "@pulumi/azure-native";

const someString = "foobar";
const typeVar = "Block";
const staticwebsite = new azure_native.storage.StorageAccountStaticWebsite("staticwebsite", {
    resourceGroupName: someString,
    accountName: someString,
});
// Safe enum
const faviconpng = new azure_native.storage.Blob("faviconpng", {
    resourceGroupName: someString,
    accountName: someString,
    containerName: someString,
    type: azure_native.storage.BlobType.Block,
});
// Output umsafe enum
const _404html = new azure_native.storage.Blob("_404html", {
    resourceGroupName: someString,
    accountName: someString,
    containerName: someString,
    type: staticwebsite.indexDocument,
});
// Unsafe enum
const another = new azure_native.storage.Blob("another", {
    resourceGroupName: someString,
    accountName: someString,
    containerName: someString,
    type: azure_native.storage.BlobType[typeVar],
});
