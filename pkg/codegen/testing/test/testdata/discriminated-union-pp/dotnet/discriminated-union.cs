using System.Collections.Generic;
using Pulumi;
using AzureNative = Pulumi.AzureNative;

return await Deployment.RunAsync(() => 
{
    var server = new AzureNative.DBforPostgreSQL.Server("server", new()
    {
        Location = "brazilsouth",
        Properties = new AzureNative.DBforPostgreSQL.Inputs.ServerPropertiesForRestoreArgs
        {
            CreateMode = "PointInTimeRestore",
            RestorePointInTime = "2017-12-14T00:00:37.467Z",
            SourceServerId = "/subscriptions/ffffffff-ffff-ffff-ffff-ffffffffffff/resourceGroups/SourceResourceGroup/providers/Microsoft.DBforPostgreSQL/servers/sourceserver",
        },
        ResourceGroupName = "TargetResourceGroup",
        ServerName = "targetserver",
        Sku = new AzureNative.DBforPostgreSQL.Inputs.SkuArgs
        {
            Capacity = 2,
            Family = "Gen5",
            Name = "B_Gen5_2",
            Tier = "Basic",
        },
        Tags = 
        {
            { "ElasticServer", "1" },
        },
    });

});

