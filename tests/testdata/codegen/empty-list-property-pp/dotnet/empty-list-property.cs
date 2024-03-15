using System.Collections.Generic;
using System.Linq;
using Pulumi;
using AzureNative = Pulumi.AzureNative;

return await Deployment.RunAsync(() => 
{
    var storageAccounts = new AzureNative.Storage.StorageAccount("storageAccounts", new()
    {
        AccountName = "sto4445",
        Kind = AzureNative.Storage.Kind.BlockBlobStorage,
        Location = "eastus",
        ResourceGroupName = "res9101",
        Sku = new AzureNative.Storage.Inputs.SkuArgs
        {
            Name = AzureNative.Storage.SkuName.Premium_LRS,
        },
        NetworkRuleSet = new AzureNative.Storage.Inputs.NetworkRuleSetArgs
        {
            DefaultAction = AzureNative.Storage.DefaultAction.Allow,
            IpRules = new() { },
        },
    });

});

