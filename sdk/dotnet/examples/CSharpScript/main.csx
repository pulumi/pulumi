#r "../PulumiAzure/bin/Debug/netstandard2.1/Pulumi.dll"
#r "../PulumiAzure/bin/Debug/netstandard2.1/PulumiAzure.dll"

using Pulumi.Azure.Core;
using Storage = Pulumi.Azure.Storage;

var resourceGroup = new ResourceGroup("rg");

var storageAccount = new Storage.Account("sa", new Storage.AccountArgs
{
    ResourceGroupName = resourceGroup.Name,
    AccountReplicationType = "LRS",
    AccountTier = "Standard",
});