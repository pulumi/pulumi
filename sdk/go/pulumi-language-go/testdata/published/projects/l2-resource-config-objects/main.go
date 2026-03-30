package main

import (
	"encoding/json"

	"example.com/pulumi-primitive/sdk/go/v7/primitive"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")
		var plainNumberArrayData []float64
		cfg.RequireObject("plainNumberArray", &plainNumberArrayData)
		plainNumberArray := pulumi.ToFloat64Array(plainNumberArrayData)
		var plainBooleanMapData map[string]bool
		cfg.RequireObject("plainBooleanMap", &plainBooleanMapData)
		plainBooleanMap := pulumi.ToBoolMap(plainBooleanMapData)
		secretNumberArray := cfg.RequireSecret("secretNumberArray").ApplyT(func(s string) ([]float64, error) {
			var vals []float64
			if err := json.Unmarshal([]byte(s), &vals); err != nil {
				return nil, err
			}
			return vals, nil
		}).(pulumi.Float64ArrayOutput)
		secretBooleanMap := cfg.RequireSecret("secretBooleanMap").ApplyT(func(s string) (map[string]bool, error) {
			var vals map[string]bool
			if err := json.Unmarshal([]byte(s), &vals); err != nil {
				return nil, err
			}
			return vals, nil
		}).(pulumi.BoolMapOutput)
		_, err := primitive.NewResource(ctx, "plain", &primitive.ResourceArgs{
			Boolean:     pulumi.Bool(true),
			Float:       pulumi.Float64(3.5),
			Integer:     pulumi.Int(3),
			String:      pulumi.String("plain"),
			NumberArray: plainNumberArray,
			BooleanMap:  pulumi.BoolMap(plainBooleanMap),
		})
		if err != nil {
			return err
		}
		_, err = primitive.NewResource(ctx, "secret", &primitive.ResourceArgs{
			Boolean:     pulumi.Bool(true),
			Float:       pulumi.Float64(3.5),
			Integer:     pulumi.Int(3),
			String:      pulumi.String("secret"),
			NumberArray: secretNumberArray,
			BooleanMap:  secretBooleanMap,
		})
		if err != nil {
			return err
		}
		return nil
	})
}
