resource frontDoor "azure-nextgen:network:FrontDoor" {
    routingRules = [{
        routeConfiguration = {
            odataType = "#Microsoft.Azure.FrontDoor.Models.FrontdoorForwardingConfiguration"
            backendPool = {
                id = "/subscriptions/subid/resourceGroups/rg1/providers/Microsoft.Network/frontDoors/frontDoor1/backendPools/backendPool1"
            }
        }
    }]
}
