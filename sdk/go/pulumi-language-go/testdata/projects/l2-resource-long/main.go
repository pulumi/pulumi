package main

import (
	"example.com/pulumi-long/sdk/go/v8/long"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		small, err := long.NewResource(ctx, "small", &long.ResourceArgs{
			Value: pulumi.NewBigInt(256),
		})
		if err != nil {
			return err
		}
		min53, err := long.NewResource(ctx, "min53", &long.ResourceArgs{
			Value: pulumi.NewBigInt(-9007199254740992),
		})
		if err != nil {
			return err
		}
		max53, err := long.NewResource(ctx, "max53", &long.ResourceArgs{
			Value: pulumi.NewBigInt(9007199254740992),
		})
		if err != nil {
			return err
		}
		min64, err := long.NewResource(ctx, "min64", &long.ResourceArgs{
			Value: pulumi.NewBigInt(-9223372036854775808),
		})
		if err != nil {
			return err
		}
		max64, err := long.NewResource(ctx, "max64", &long.ResourceArgs{
			Value: pulumi.NewBigInt(9223372036854775807),
		})
		if err != nil {
			return err
		}
		_uint64, err := long.NewResource(ctx, "uint64", &long.ResourceArgs{
			Value: pulumi.MustParseBigInt("18446744073709551615"),
		})
		if err != nil {
			return err
		}
		huge, err := long.NewResource(ctx, "huge", &long.ResourceArgs{
			Value: pulumi.MustParseBigInt("20000000000000000001"),
		})
		if err != nil {
			return err
		}
		ctx.Export("huge", pulumi.MustParseBigInt("20000000000000000001"))
		ctx.Export("roundtrip", huge.Value)
		ctx.Export("result", pulumi.All(small.Value, min53.Value, max53.Value, min64.Value, max64.Value, _uint64.Value, huge.Value).ApplyT(func(_args []interface{}) (pulumi.BigInt, error) {
			smallValue := _args[0].(pulumi.BigInt)
			min53Value := _args[1].(pulumi.BigInt)
			max53Value := _args[2].(pulumi.BigInt)
			min64Value := _args[3].(pulumi.BigInt)
			max64Value := _args[4].(pulumi.BigInt)
			uint64Value := _args[5].(pulumi.BigInt)
			hugeValue := _args[6].(pulumi.BigInt)
			return pulumi.BigInt((((((smallValue.Add(min53Value)).Add(max53Value)).Add(min64Value)).Add(max64Value)).Add(uint64Value)).Add(hugeValue)), nil
		}).(pulumi.BigIntOutput))
		return nil
	})
}
