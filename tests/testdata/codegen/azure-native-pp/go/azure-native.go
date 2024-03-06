package main

import (
	"github.com/pulumi/pulumi-azure-native/sdk/go/azure/cdn"
	"github.com/pulumi/pulumi-azure-native/sdk/go/azure/network"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := network.NewFrontDoor(ctx, "frontDoor", &network.FrontDoorArgs{
			ResourceGroupName: pulumi.String("someGroupName"),
			RoutingRules: network.RoutingRuleArray{
				&network.RoutingRuleArgs{
					RouteConfiguration: network.ForwardingConfiguration{
						OdataType: "#Microsoft.Azure.FrontDoor.Models.FrontdoorForwardingConfiguration",
						BackendPool: network.SubResource{
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
			Origins: cdn.DeepCreatedOriginArray{},
			DeliveryPolicy: &cdn.EndpointPropertiesUpdateParametersDeliveryPolicyArgs{
				Rules: []cdn.DeliveryRuleArgs{
					{
						Actions: pulumi.Array{
							{
								Name: "CacheExpiration",
								Parameters: {
									CacheBehavior: "Override",
									CacheDuration: "10:10:09",
									CacheType:     "All",
									OdataType:     "#Microsoft.Azure.Cdn.Models.DeliveryRuleCacheExpirationActionParameters",
								},
							},
							{
								Name: "ModifyResponseHeader",
								Parameters: {
									HeaderAction: "Overwrite",
									HeaderName:   "Access-Control-Allow-Origin",
									OdataType:    "#Microsoft.Azure.Cdn.Models.DeliveryRuleHeaderActionParameters",
									Value:        "*",
								},
							},
							{
								Name: "ModifyRequestHeader",
								Parameters: {
									HeaderAction: "Overwrite",
									HeaderName:   "Accept-Encoding",
									OdataType:    "#Microsoft.Azure.Cdn.Models.DeliveryRuleHeaderActionParameters",
									Value:        "gzip",
								},
							},
						},
						Conditions: pulumi.Array{
							{
								Name: "RemoteAddress",
								Parameters: {
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
			ProfileName:          pulumi.String("profileName"),
			ResourceGroupName:    pulumi.String("resourceGroupName"),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
