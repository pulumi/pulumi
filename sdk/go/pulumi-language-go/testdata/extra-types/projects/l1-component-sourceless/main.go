package main

import (
	"encoding/json"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		tmpJSON0, err := json.Marshal(map[string]string{
			"key": "value",
		})
		if err != nil {
			return err
		}
		json0 := string(tmpJSON0)
		myComponent := &pulumi.ResourceState{}
		err = ctx.RegisterComponentResourceV2("my:custom:Component", "myComponent", pulumi.Map{
			"aNumber": pulumi.Any(42),
			"aString": pulumi.Any("hello"),
			"aJson":   pulumi.Any(json0),
		}, myComponent)
		if err != nil {
			return err
		}
		return nil
	})
}
