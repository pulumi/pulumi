package main

import (
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
	cdn "github.com/pulumi/pulumi-azure-nextgen/sdk/go/azure/cdn"
	network "github.com/pulumi/pulumi-azure-nextgen/sdk/go/azure/network"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := network.NewFrontDoor(ctx, "frontDoor", &network.FrontDoorArgs{
			RoutingRules: network.RoutingRuleArray{
				&network.RoutingRuleArgs{
					RouteConfiguration: &network.ForwardingConfigurationArgs{
						OdataType: pulumi.String("#Microsoft.Azure.FrontDoor.Models.FrontdoorForwardingConfiguration"),
						BackendPool: &network.SubResourceArgs{
							Id: pulumi.String("/subscriptions/subid/resourceGroups/rg1/providers/Microsoft.Network/frontDoors/frontDoor1/backendPools/backendPool1"),
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
						Actions: {
							&cdn.DeliveryRuleCacheExpirationActionArgs{
								Name: pulumi.String("CacheExpiration"),
								Parameters: &cdn.CacheExpirationActionParametersArgs{
									CacheBehavior: pulumi.String("Override"),
									CacheDuration: pulumi.String("10:10:09"),
									CacheType:     pulumi.String("All"),
									OdataType:     pulumi.String("#Microsoft.Azure.Cdn.Models.DeliveryRuleCacheExpirationActionParameters"),
								},
							},
							&cdn.DeliveryRuleResponseHeaderActionArgs{
								Name: pulumi.String("ModifyResponseHeader"),
								Parameters: &cdn.HeaderActionParametersArgs{
									HeaderAction: pulumi.String("Overwrite"),
									HeaderName:   pulumi.String("Access-Control-Allow-Origin"),
									OdataType:    pulumi.String("#Microsoft.Azure.Cdn.Models.DeliveryRuleHeaderActionParameters"),
									Value:        pulumi.String("*"),
								},
							},
							&cdn.DeliveryRuleRequestHeaderActionArgs{
								Name: pulumi.String("ModifyRequestHeader"),
								Parameters: &cdn.HeaderActionParametersArgs{
									HeaderAction: pulumi.String("Overwrite"),
									HeaderName:   pulumi.String("Accept-Encoding"),
									OdataType:    pulumi.String("#Microsoft.Azure.Cdn.Models.DeliveryRuleHeaderActionParameters"),
									Value:        pulumi.String("gzip"),
								},
							},
						},
						Conditions: {
							&cdn.DeliveryRuleRemoteAddressConditionArgs{
								Name: pulumi.String("RemoteAddress"),
								Parameters: &cdn.RemoteAddressMatchConditionParametersArgs{
									MatchValues: pulumi.StringArray{
										pulumi.String("192.168.1.0/24"),
										pulumi.String("10.0.0.0/24"),
									},
									NegateCondition: pulumi.Bool(true),
									OdataType:       pulumi.String("#Microsoft.Azure.Cdn.Models.DeliveryRuleRemoteAddressConditionParameters"),
									Operator:        pulumi.String("IPMatch"),
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
