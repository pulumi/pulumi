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
        var endpoint = new AzureNextGen.Endpoint("endpoint", new AzureNextGen.EndpointArgs
        {
            DeliveryPolicy = new AzureNextGen.Inputs.EndpointPropertiesUpdateParametersDeliveryPolicyArgs
            {
                Rules = 
                {
                    new AzureNextGen.Inputs.DeliveryRuleArgs
                    {
                        Actions = 
                        {
                            new AzureNextGen.Inputs.DeliveryRuleCacheExpirationActionArgs
                            {
                                Name = "CacheExpiration",
                                Parameters = new AzureNextGen.Inputs.CacheExpirationActionParametersArgs
                                {
                                    CacheBehavior = "Override",
                                    CacheDuration = "10:10:09",
                                    CacheType = "All",
                                    OdataType = "#Microsoft.Azure.Cdn.Models.DeliveryRuleCacheExpirationActionParameters",
                                },
                            },
                            new AzureNextGen.Inputs.DeliveryRuleResponseHeaderActionArgs
                            {
                                Name = "ModifyResponseHeader",
                                Parameters = new AzureNextGen.Inputs.HeaderActionParametersArgs
                                {
                                    HeaderAction = "Overwrite",
                                    HeaderName = "Access-Control-Allow-Origin",
                                    OdataType = "#Microsoft.Azure.Cdn.Models.DeliveryRuleHeaderActionParameters",
                                    Value = "*",
                                },
                            },
                            new AzureNextGen.Inputs.DeliveryRuleRequestHeaderActionArgs
                            {
                                Name = "ModifyRequestHeader",
                                Parameters = new AzureNextGen.Inputs.HeaderActionParametersArgs
                                {
                                    HeaderAction = "Overwrite",
                                    HeaderName = "Accept-Encoding",
                                    OdataType = "#Microsoft.Azure.Cdn.Models.DeliveryRuleHeaderActionParameters",
                                    Value = "gzip",
                                },
                            },
                        },
                        Conditions = 
                        {
                            new AzureNextGen.Inputs.DeliveryRuleRemoteAddressConditionArgs
                            {
                                Name = "RemoteAddress",
                                Parameters = new AzureNextGen.Inputs.RemoteAddressMatchConditionParametersArgs
                                {
                                    MatchValues = 
                                    {
                                        "192.168.1.0/24",
                                        "10.0.0.0/24",
                                    },
                                    NegateCondition = true,
                                    OdataType = "#Microsoft.Azure.Cdn.Models.DeliveryRuleRemoteAddressConditionParameters",
                                    Operator = "IPMatch",
                                },
                            },
                        },
                        Name = "rule1",
                        Order = 1,
                    },
                },
            },
            EndpointName = "endpoint1",
            IsCompressionEnabled = true,
            IsHttpAllowed = true,
            IsHttpsAllowed = true,
            Location = "WestUs",
        });
    }

}
