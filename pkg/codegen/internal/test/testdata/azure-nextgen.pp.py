import pulumi
import pulumi_azure_nextgen as azure_nextgen

front_door = azure_nextgen.FrontDoor("frontDoor", routing_rules=[azure_nextgen.network.RoutingRuleArgs(
    route_configuration={
        "odata_type": "#Microsoft.Azure.FrontDoor.Models.FrontdoorForwardingConfiguration",
        "backend_pool": {
            "id": "/subscriptions/subid/resourceGroups/rg1/providers/Microsoft.Network/frontDoors/frontDoor1/backendPools/backendPool1",
        },
    },
)])
