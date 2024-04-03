using System.Collections.Generic;
using System.Linq;
using Pulumi;
using AzureNative = Pulumi.AzureNative;

return await Deployment.RunAsync(() => 
{
    var frontDoor = new AzureNative.Network.FrontDoor("frontDoor", new()
    {
        ResourceGroupName = "someGroupName",
        RoutingRules = new[]
        {
            new AzureNative.Network.Inputs.RoutingRuleArgs
            {
                RouteConfiguration = new AzureNative.Network.Inputs.ForwardingConfigurationArgs
                {
                    OdataType = "#Microsoft.Azure.FrontDoor.Models.FrontdoorForwardingConfiguration",
                    BackendPool = new AzureNative.Network.Inputs.SubResourceArgs
                    {
                        Id = "/subscriptions/subid/resourceGroups/rg1/providers/Microsoft.Network/frontDoors/frontDoor1/backendPools/backendPool1",
                    },
                },
            },
        },
    });

    var endpoint = new AzureNative.Cdn.Endpoint("endpoint", new()
    {
        Origins = new[] {},
        DeliveryPolicy = new AzureNative.Cdn.Inputs.EndpointPropertiesUpdateParametersDeliveryPolicyArgs
        {
            Rules = new[]
            {
                new AzureNative.Cdn.Inputs.DeliveryRuleArgs
                {
                    Actions = 
                    {
                        new AzureNative.Cdn.Inputs.DeliveryRuleCacheExpirationActionArgs
                        {
                            Name = "CacheExpiration",
                            Parameters = new AzureNative.Cdn.Inputs.CacheExpirationActionParametersArgs
                            {
                                CacheBehavior = AzureNative.Cdn.CacheBehavior.Override,
                                CacheDuration = "10:10:09",
                                CacheType = AzureNative.Cdn.CacheType.All,
                                OdataType = "#Microsoft.Azure.Cdn.Models.DeliveryRuleCacheExpirationActionParameters",
                            },
                        },
                        new AzureNative.Cdn.Inputs.DeliveryRuleResponseHeaderActionArgs
                        {
                            Name = "ModifyResponseHeader",
                            Parameters = new AzureNative.Cdn.Inputs.HeaderActionParametersArgs
                            {
                                HeaderAction = AzureNative.Cdn.HeaderAction.Overwrite,
                                HeaderName = "Access-Control-Allow-Origin",
                                OdataType = "#Microsoft.Azure.Cdn.Models.DeliveryRuleHeaderActionParameters",
                                Value = "*",
                            },
                        },
                        new AzureNative.Cdn.Inputs.DeliveryRuleRequestHeaderActionArgs
                        {
                            Name = "ModifyRequestHeader",
                            Parameters = new AzureNative.Cdn.Inputs.HeaderActionParametersArgs
                            {
                                HeaderAction = AzureNative.Cdn.HeaderAction.Overwrite,
                                HeaderName = "Accept-Encoding",
                                OdataType = "#Microsoft.Azure.Cdn.Models.DeliveryRuleHeaderActionParameters",
                                Value = "gzip",
                            },
                        },
                    },
                    Conditions = new[]
                    {
                        new AzureNative.Cdn.Inputs.DeliveryRuleRemoteAddressConditionArgs
                        {
                            Name = "RemoteAddress",
                            Parameters = new AzureNative.Cdn.Inputs.RemoteAddressMatchConditionParametersArgs
                            {
                                MatchValues = new[]
                                {
                                    "192.168.1.0/24",
                                    "10.0.0.0/24",
                                },
                                NegateCondition = true,
                                OdataType = "#Microsoft.Azure.Cdn.Models.DeliveryRuleRemoteAddressConditionParameters",
                                Operator = AzureNative.Cdn.RemoteAddressOperator.IPMatch,
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
        ProfileName = "profileName",
        ResourceGroupName = "resourceGroupName",
    });

});

