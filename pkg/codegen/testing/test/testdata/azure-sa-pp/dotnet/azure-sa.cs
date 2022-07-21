using System.Collections.Generic;
using Pulumi;
using Azure = Pulumi.Azure;

return await Deployment.RunAsync(() => 
{
    var config = new Config();
    var storageAccountNameParam = config.Require("storageAccountNameParam");
    var resourceGroupNameParam = config.Require("resourceGroupNameParam");
    var resourceGroupVar = Azure.Core.GetResourceGroup.Invoke(new()
    {
        Name = resourceGroupNameParam,
    });

    var locationParam = Output.Create(config.Get("locationParam")) ?? resourceGroupVar.Apply(getResourceGroupResult => getResourceGroupResult.Location);
    var storageAccountTierParam = config.Get("storageAccountTierParam") ?? "Standard";
    var storageAccountTypeReplicationParam = config.Get("storageAccountTypeReplicationParam") ?? "LRS";
    var storageAccountResource = new Azure.Storage.Account("storageAccountResource", new()
    {
        Name = storageAccountNameParam,
        AccountKind = "StorageV2",
        Location = locationParam,
        ResourceGroupName = resourceGroupNameParam,
        AccountTier = storageAccountTierParam,
        AccountReplicationType = storageAccountTypeReplicationParam,
    });

    return new Dictionary<string, object?>
    {
        ["storageAccountNameOut"] = storageAccountResource.Name,
    };
});

