package main

import (
	"example.com/pulumi-plain/sdk/go/v13/plain"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := plain.NewResource(ctx, "res", &plain.ResourceArgs{
			Data: plain.DataArgs{
				InnerData: plain.InnerDataArgs{
					Boolean: false,
					Float:   2.17,
					Integer: -12,
					String:  "Goodbye",
					BoolArray: []bool{
						false,
						true,
					},
					StringMap: map[string]string{
						"two":   "turtle doves",
						"three": "french hens",
					},
				},
				Boolean: true,
				Float:   4.5,
				Integer: 1024,
				String:  "Hello",
				BoolArray: []bool{
					true,
					false,
				},
				StringMap: map[string]string{
					"x": "100",
					"y": "200",
				},
			},
		})
		if err != nil {
			return err
		}
		_, err = plain.NewResource(ctx, "emptyListRes", &plain.ResourceArgs{
			Data: plain.DataArgs{
				InnerData: plain.InnerDataArgs{
					Boolean:   false,
					Float:     0,
					Integer:   0,
					String:    "",
					BoolArray: []bool{},
					StringMap: map[string]string{},
				},
				Boolean:   false,
				Float:     0,
				Integer:   0,
				String:    "",
				BoolArray: []bool{},
				StringMap: map[string]string{},
			},
			DataList: []plain.InnerDataArgs{},
		})
		if err != nil {
			return err
		}
		return nil
	})
}
