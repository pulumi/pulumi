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
		var multipleSimpleComponents []*SimpleComponent
		for index := 0; index < 10; index++ {
			key0 := index
			_ := index
			__res, err := NewSimpleComponent(ctx, fmt.Sprintf("multipleSimpleComponents-%v", key0), nil)
			if err != nil {
				return err
			}
			multipleSimpleComponents = append(multipleSimpleComponents, __res)
		}
		_, err = NewAnotherComponent(ctx, "anotherComponent", nil)
		if err != nil {
			return err
		}
		exampleComponent, err := NewExampleComponent(ctx, "exampleComponent", &ExampleComponentArgs{
			Input: "doggo",
			IpAddress: pulumi.IntArray{
				pulumi.Int(127),
				pulumi.Int(0),
				pulumi.Int(0),
				pulumi.Int(1),
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
