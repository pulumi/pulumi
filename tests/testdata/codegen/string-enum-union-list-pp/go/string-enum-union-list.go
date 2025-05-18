package main

import (
	"github.com/pulumi/pulumi-azure-native/sdk/go/azure/servicebus"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := servicebus.NewNamespaceAuthorizationRule(ctx, "namespaceAuthorizationRule", &servicebus.NamespaceAuthorizationRuleArgs{
			AuthorizationRuleName: "sdk-AuthRules-1788",
			NamespaceName:         pulumi.String("sdk-Namespace-6914"),
			ResourceGroupName:     pulumi.String("ArunMonocle"),
			Rights: pulumi.StringArray{
				pulumi.String(servicebus.AccessRightsListen),
				pulumi.String(servicebus.AccessRightsSend),
			},
		})
		if err != nil {
			return err
		}
		return nil
	})
}
