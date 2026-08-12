package main

import (
	"fmt"

	"example.com/pulumi-nestedobject/sdk/go/nestedobject"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")
		prefix := cfg.Require("prefix")
		var item []*nestedobject.Target
		for index := 0; index < 2; index++ {
			key0 := index
			val0 := index
			__res, err := nestedobject.NewTarget(ctx, fmt.Sprintf("item-%v", key0), &nestedobject.TargetArgs{
				Name: pulumi.Sprintf("%v-%v", prefix, val0),
			})
			if err != nil {
				return err
			}
			item = append(item, __res)
		}
		return nil
	})
}
