import * as pulumi from "@pulumi/pulumi";
import * as azure_nextgen from "@pulumi/azure-nextgen";

const frontDoor = new azure_nextgen.FrontDoor("frontDoor", {routingRules: [{
    routeConfiguration: {
        odataType: "#Microsoft.Azure.FrontDoor.Models.FrontdoorForwardingConfiguration",
        backendPool: {
            id: "/subscriptions/subid/resourceGroups/rg1/providers/Microsoft.Network/frontDoors/frontDoor1/backendPools/backendPool1",
        },
    },
}]});
