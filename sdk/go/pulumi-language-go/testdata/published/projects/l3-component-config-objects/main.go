package main

import (
	"encoding/json"

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
		_, err := NewPrimitiveComponent(ctx, "plain", &PrimitiveComponentArgs{
			NumberArray: plainNumberArray,
			BooleanMap:  plainBooleanMap,
		})
		if err != nil {
			return err
		}
		_, err = NewPrimitiveComponent(ctx, "secret", &PrimitiveComponentArgs{
			NumberArray: secretNumberArray,
			BooleanMap:  secretBooleanMap,
		})
		if err != nil {
			return err
		}
		return nil
	})
}
