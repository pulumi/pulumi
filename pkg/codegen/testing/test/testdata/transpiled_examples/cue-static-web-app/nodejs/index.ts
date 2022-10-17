import * as pulumi from "@pulumi/pulumi";
import * as azure_native from "@pulumi/azure-native";

const rawkodeGroup = new azure_native.resources.ResourceGroup("rawkode-group", {location: "WestUs"});
const rawkodeStorage = new azure_native.storage.StorageAccount("rawkode-storage", {
    resourceGroupName: rawkodeGroup.name,
    kind: "StorageV2",
    sku: {
        name: "Standard_LRS",
    },
});
const rawkodeWebsite = new azure_native.storage.StorageAccountStaticWebsite("rawkode-website", {
    resourceGroupName: rawkodeGroup.name,
    accountName: rawkodeStorage.name,
    indexDocument: "index.html",
    error404Document: "404.html",
});
const rawkodeIndexHtml = new azure_native.storage.Blob("rawkode-index.html", {
    resourceGroupName: rawkodeGroup.name,
    accountName: rawkodeStorage.name,
    containerName: rawkodeWebsite.containerName,
    contentType: "text/html",
    type: azure_native.storage.BlobType.Block,
    source: new pulumi.asset.FileAsset("./website/index.html"),
});
const stack72Group = new azure_native.resources.ResourceGroup("stack72-group", {location: "WestUs"});
const stack72Storage = new azure_native.storage.StorageAccount("stack72-storage", {
    resourceGroupName: stack72Group.name,
    kind: "StorageV2",
    sku: {
        name: "Standard_LRS",
    },
});
const stack72Website = new azure_native.storage.StorageAccountStaticWebsite("stack72-website", {
    resourceGroupName: stack72Group.name,
    accountName: stack72Storage.name,
    indexDocument: "index.html",
    error404Document: "404.html",
});
const stack72IndexHtml = new azure_native.storage.Blob("stack72-index.html", {
    resourceGroupName: stack72Group.name,
    accountName: stack72Storage.name,
    containerName: stack72Website.containerName,
    contentType: "text/html",
    type: azure_native.storage.BlobType.Block,
    source: new pulumi.asset.FileAsset("./website/index.html"),
});
