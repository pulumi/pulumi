import * as pulumi from "@pulumi/pulumi";
import * as azure_native from "@pulumi/azure-native";

const server = new azure_native.dbforpostgresql.Server("server", {
    location: "brazilsouth",
    properties: {
        createMode: "PointInTimeRestore",
        restorePointInTime: "2017-12-14T00:00:37.467Z",
        sourceServerId: "/subscriptions/ffffffff-ffff-ffff-ffff-ffffffffffff/resourceGroups/SourceResourceGroup/providers/Microsoft.DBforPostgreSQL/servers/sourceserver",
    },
    resourceGroupName: "TargetResourceGroup",
    serverName: "targetserver",
    sku: {
        capacity: 2,
        family: "Gen5",
        name: "B_Gen5_2",
        tier: "Basic",
    },
    tags: {
        ElasticServer: "1",
    },
});
