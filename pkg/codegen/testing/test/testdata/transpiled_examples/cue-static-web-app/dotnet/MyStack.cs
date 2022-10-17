using Pulumi;
using AzureNative = Pulumi.AzureNative;

class MyStack : Stack
{
    public MyStack()
    {
        var rawkodeGroup = new AzureNative.Resources.ResourceGroup("rawkode-group", new AzureNative.Resources.ResourceGroupArgs
        {
            Location = "WestUs",
        });
        var rawkodeStorage = new AzureNative.Storage.StorageAccount("rawkode-storage", new AzureNative.Storage.StorageAccountArgs
        {
            ResourceGroupName = rawkodeGroup.Name,
            Kind = "StorageV2",
            Sku = new AzureNative.Storage.Inputs.SkuArgs
            {
                Name = "Standard_LRS",
            },
        });
        var rawkodeWebsite = new AzureNative.Storage.StorageAccountStaticWebsite("rawkode-website", new AzureNative.Storage.StorageAccountStaticWebsiteArgs
        {
            ResourceGroupName = rawkodeGroup.Name,
            AccountName = rawkodeStorage.Name,
            IndexDocument = "index.html",
            Error404Document = "404.html",
        });
        var rawkodeIndexHtml = new AzureNative.Storage.Blob("rawkode-index.html", new AzureNative.Storage.BlobArgs
        {
            ResourceGroupName = rawkodeGroup.Name,
            AccountName = rawkodeStorage.Name,
            ContainerName = rawkodeWebsite.ContainerName,
            ContentType = "text/html",
            Type = AzureNative.Storage.BlobType.Block,
            Source = new FileAsset("./website/index.html"),
        });
        var stack72Group = new AzureNative.Resources.ResourceGroup("stack72-group", new AzureNative.Resources.ResourceGroupArgs
        {
            Location = "WestUs",
        });
        var stack72Storage = new AzureNative.Storage.StorageAccount("stack72-storage", new AzureNative.Storage.StorageAccountArgs
        {
            ResourceGroupName = stack72Group.Name,
            Kind = "StorageV2",
            Sku = new AzureNative.Storage.Inputs.SkuArgs
            {
                Name = "Standard_LRS",
            },
        });
        var stack72Website = new AzureNative.Storage.StorageAccountStaticWebsite("stack72-website", new AzureNative.Storage.StorageAccountStaticWebsiteArgs
        {
            ResourceGroupName = stack72Group.Name,
            AccountName = stack72Storage.Name,
            IndexDocument = "index.html",
            Error404Document = "404.html",
        });
        var stack72IndexHtml = new AzureNative.Storage.Blob("stack72-index.html", new AzureNative.Storage.BlobArgs
        {
            ResourceGroupName = stack72Group.Name,
            AccountName = stack72Storage.Name,
            ContainerName = stack72Website.ContainerName,
            ContentType = "text/html",
            Type = AzureNative.Storage.BlobType.Block,
            Source = new FileAsset("./website/index.html"),
        });
    }

}
