using System.Collections.Generic;
using System.Linq;
using Pulumi;
using AzureNative = Pulumi.AzureNative;

return await Deployment.RunAsync(() => 
{
    var someString = "foobar";

    var typeVar = "Block";

    var staticwebsite = new AzureNative.Storage.StorageAccountStaticWebsite("staticwebsite", new()
    {
        ResourceGroupName = someString,
        AccountName = someString,
    });

    // Safe enum
    var faviconpng = new AzureNative.Storage.Blob("faviconpng", new()
    {
        ResourceGroupName = someString,
        AccountName = someString,
        ContainerName = someString,
        Type = AzureNative.Storage.BlobType.Block,
    });

    // Output umsafe enum
    var _404html = new AzureNative.Storage.Blob("_404html", new()
    {
        ResourceGroupName = someString,
        AccountName = someString,
        ContainerName = someString,
        Type = staticwebsite.IndexDocument.Apply(System.Enum.Parse<AzureNative.Storage.BlobType>),
    });

    // Unsafe enum
    var another = new AzureNative.Storage.Blob("another", new()
    {
        ResourceGroupName = someString,
        AccountName = someString,
        ContainerName = someString,
        Type = System.Enum.Parse<AzureNative.Storage.BlobType>(typeVar),
    });

});

