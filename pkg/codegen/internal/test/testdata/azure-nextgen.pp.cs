using Pulumi;
using AzureNextGen = Pulumi.AzureNextGen;

class MyStack : Stack
{
    public MyStack()
    {
        var frontDoor = new AzureNextGen.FrontDoor("frontDoor", new AzureNextGen.FrontDoorArgs
        {
            RoutingRules = 
            {
                new AzureNextGen.Inputs.RoutingRuleArgs
                {
                    RouteConfiguration = new AzureNextGen.Inputs.ForwardingConfigurationArgs
                    {
                        OdataType = "#Microsoft.Azure.FrontDoor.Models.FrontdoorForwardingConfiguration",
                        BackendPool = new AzureNextGen.Inputs.SubResourceArgs
                        {
                            Id = "/subscriptions/subid/resourceGroups/rg1/providers/Microsoft.Network/frontDoors/frontDoor1/backendPools/backendPool1",
                        },
                    },
                },
            },
        });
    }

}
