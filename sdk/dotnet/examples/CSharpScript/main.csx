#r "../PulumiAzure/bin/Debug/netstandard2.1/Pulumi.dll"
#r "../PulumiAzure/bin/Debug/netstandard2.1/PulumiAzure.dll"

using Pulumi.Azure.Core;
using Pulumi.Azure.Storage;

var resourceGroup = new ResourceGroup("rg");

var storageAccount = new Account("sa", new AccountArgs
{
    ResourceGroupName = resourceGroup.Name,
    AccountReplicationType = "LRS",
    AccountTier = "Standard",
});