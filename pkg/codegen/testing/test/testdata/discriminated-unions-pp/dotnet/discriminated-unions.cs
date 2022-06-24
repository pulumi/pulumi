using Pulumi;
using AzureNative = Pulumi.AzureNative;

class MyStack : Stack
{
    public MyStack()
    {
        var databaseAccount = new AzureNative.DocumentDB.DatabaseAccount("databaseAccount", new AzureNative.DocumentDB.DatabaseAccountArgs
        {
            AccountName = "ddb1",
            ApiProperties = new AzureNative.DocumentDB.Inputs.ApiPropertiesArgs
            {
                ServerVersion = "3.2",
            },
            BackupPolicy = 
            {
                { "periodicModeProperties", new AzureNative.DocumentDB.Inputs.PeriodicModePropertiesArgs
                {
                    BackupIntervalInMinutes = 240,
                    BackupRetentionIntervalInHours = 8,
                } },
                { "type", "Periodic" },
            },
            DatabaseAccountOfferType = AzureNative.DocumentDB.DatabaseAccountOfferType.Standard,
            Locations = 
            {
                new AzureNative.DocumentDB.Inputs.LocationArgs
                {
                    FailoverPriority = 0,
                    IsZoneRedundant = false,
                    LocationName = "sourthcentralus",
                },
            },
            ResourceGroupName = "rg1",
        });
    }

}
