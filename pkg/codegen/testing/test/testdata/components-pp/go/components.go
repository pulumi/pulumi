package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := NewSimpleComponent(ctx, "simpleComponent", nil)
		if err != nil {
			return err
		}
		exampleComponent, err := NewExampleComponent(ctx, "exampleComponent", &ExampleComponentArgs{
			Input: "doggo",
			IpAddress: []int{
				127,
				0,
				0,
				1,
			},
			CidrBlocks: map[string]interface{}{
				"one": "uno",
				"two": "dos",
			},
			GithubApp: &GithubAppArgs{
				Id:            "example id",
				KeyBase64:     "base64 encoded key",
				WebhookSecret: "very important secret",
			},
			Servers: []map[string]interface{}{
				&ServersArgs{
					Name: "First",
				},
				&ServersArgs{
					Name: "Second",
				},
			},
			DeploymentZones: map[string]interface{}{
				"first": &DeploymentZonesArgs{
					Zone: "First zone",
				},
				"second": &DeploymentZonesArgs{
					Zone: "Second zone",
				},
			},
		})
		if err != nil {
			return err
		}
		ctx.Export("result", exampleComponent.Result)
		return nil
	})
}
