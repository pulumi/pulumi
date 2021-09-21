import * as pulumi from "@pulumi/pulumi";
import * as azure_native from "@pulumi/azure-native";

const frontDoor = new azure_native.network.FrontDoor("frontDoor", {routingRules: [{
    routeConfiguration: {
        odataType: "#Microsoft.Azure.FrontDoor.Models.FrontdoorForwardingConfiguration",
        backendPool: {
            id: "/subscriptions/subid/resourceGroups/rg1/providers/Microsoft.Network/frontDoors/frontDoor1/backendPools/backendPool1",
        },
    },
}]});
const endpoint = new azure_native.cdn.Endpoint("endpoint", {
    deliveryPolicy: {
        rules: [{
            actions: [
                {
                    name: "CacheExpiration",
                    parameters: {
                        cacheBehavior: "Override",
                        cacheDuration: "10:10:09",
                        cacheType: "All",
                        odataType: "#Microsoft.Azure.Cdn.Models.DeliveryRuleCacheExpirationActionParameters",
                    },
                },
                {
                    name: "ModifyResponseHeader",
                    parameters: {
                        headerAction: "Overwrite",
                        headerName: "Access-Control-Allow-Origin",
                        odataType: "#Microsoft.Azure.Cdn.Models.DeliveryRuleHeaderActionParameters",
                        value: "*",
                    },
                },
                {
                    name: "ModifyRequestHeader",
                    parameters: {
                        headerAction: "Overwrite",
                        headerName: "Accept-Encoding",
                        odataType: "#Microsoft.Azure.Cdn.Models.DeliveryRuleHeaderActionParameters",
                        value: "gzip",
                    },
                },
            ],
            conditions: [{
                name: "RemoteAddress",
                parameters: {
                    matchValues: [
                        "192.168.1.0/24",
                        "10.0.0.0/24",
                    ],
                    negateCondition: true,
                    odataType: "#Microsoft.Azure.Cdn.Models.DeliveryRuleRemoteAddressConditionParameters",
                    operator: "IPMatch",
                },
            }],
            name: "rule1",
            order: 1,
        }],
    },
    endpointName: "endpoint1",
    isCompressionEnabled: true,
    isHttpAllowed: true,
    isHttpsAllowed: true,
    location: "WestUs",
});
