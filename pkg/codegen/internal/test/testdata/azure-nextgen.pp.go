package main

import (
	"github.com/pulumi/pulumi-azure-nextgen/sdk/go/azure-nextgen"
	network "github.com/pulumi/pulumi-azure-nextgen/sdk/go/azure/network"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := azure - nextgen.NewFrontDoor(ctx, "frontDoor", &azure-nextgen.FrontDoorArgs{
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
		return nil
	})
}
