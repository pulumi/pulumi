package main

import (
	cdn "github.com/pulumi/pulumi-azure-native/sdk/go/azure/cdn"
	network "github.com/pulumi/pulumi-azure-native/sdk/go/azure/network"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := network.NewFrontDoor(ctx, "frontDoor", &network.FrontDoorArgs{
			RoutingRules: network.RoutingRuleArray{
				&network.RoutingRuleArgs{
					RouteConfiguration: &network.ForwardingConfigurationArgs{
						OdataType: "#Microsoft.Azure.FrontDoor.Models.FrontdoorForwardingConfiguration",
						BackendPool: &network.SubResourceArgs{
							Id: "/subscriptions/subid/resourceGroups/rg1/providers/Microsoft.Network/frontDoors/frontDoor1/backendPools/backendPool1",
						},
					},
				},
			},
		})
		if err != nil {
			return err
		}
		_, err = cdn.NewEndpoint(ctx, "endpoint", &cdn.EndpointArgs{
			DeliveryPolicy: &cdn.EndpointPropertiesUpdateParametersDeliveryPolicyArgs{
				Rules: cdn.DeliveryRuleArray{
					&cdn.DeliveryRuleArgs{
						Actions: pulumi.Array{
							&cdn.DeliveryRuleCacheExpirationActionArgs{
								Name: "CacheExpiration",
								Parameters: &cdn.CacheExpirationActionParametersArgs{
									CacheBehavior: "Override",
									CacheDuration: "10:10:09",
									CacheType:     "All",
									OdataType:     "#Microsoft.Azure.Cdn.Models.DeliveryRuleCacheExpirationActionParameters",
								},
							},
							&cdn.DeliveryRuleResponseHeaderActionArgs{
								Name: "ModifyResponseHeader",
								Parameters: &cdn.HeaderActionParametersArgs{
									HeaderAction: "Overwrite",
									HeaderName:   "Access-Control-Allow-Origin",
									OdataType:    "#Microsoft.Azure.Cdn.Models.DeliveryRuleHeaderActionParameters",
									Value:        "*",
								},
							},
							&cdn.DeliveryRuleRequestHeaderActionArgs{
								Name: "ModifyRequestHeader",
								Parameters: &cdn.HeaderActionParametersArgs{
									HeaderAction: "Overwrite",
									HeaderName:   "Accept-Encoding",
									OdataType:    "#Microsoft.Azure.Cdn.Models.DeliveryRuleHeaderActionParameters",
									Value:        "gzip",
								},
							},
						},
						Conditions: pulumi.Array{
							&cdn.DeliveryRuleRemoteAddressConditionArgs{
								Name: "RemoteAddress",
								Parameters: &cdn.RemoteAddressMatchConditionParametersArgs{
									MatchValues: []string{
										"192.168.1.0/24",
										"10.0.0.0/24",
									},
									NegateCondition: true,
									OdataType:       "#Microsoft.Azure.Cdn.Models.DeliveryRuleRemoteAddressConditionParameters",
									Operator:        "IPMatch",
								},
							},
						},
						Name:  pulumi.String("rule1"),
						Order: pulumi.Int(1),
					},
				},
			},
			EndpointName:         pulumi.String("endpoint1"),
			IsCompressionEnabled: pulumi.Bool(true),
			IsHttpAllowed:        pulumi.Bool(true),
			IsHttpsAllowed:       pulumi.Bool(true),
			Location:             pulumi.String("WestUs"),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
