package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		secondPasswordLength, resolveSecondPasswordLength := pulumi.DeferredOutput(ctx)
		first, err := NewFirst(ctx, "first", &FirstArgs{
			PasswordLength: secondPasswordLength,
		})
		if err != nil {
			return err
		}
		second, err := NewSecond(ctx, "second", &SecondArgs{
			PetName: first.PetName,
		})
		if err != nil {
			return err
		}
		resolveSecondPasswordLength(second.PasswordLength)
		loopingOverMany, resolveLoopingOverMany := pulumi.DeferredOutput(ctx)
		another, err := NewFirst(ctx, "another", &FirstArgs{
			PasswordLength: float64(loopingOverMany.ApplyT(func(loopingOverMany []int) (pulumi.Int, error) {
				return pulumi.Int(len(loopingOverMany)), nil
			}).(pulumi.IntOutput)),
		})
		if err != nil {
			return err
		}
		var many []*Second
		for index := 0; index < 10; index++ {
			key0 := index
			_ := index
			__res, err := NewSecond(ctx, fmt.Sprintf("many-%v", key0), &SecondArgs{
				PetName: another.PetName,
			})
			if err != nil {
				return err
			}
			many = append(many, __res)
		}
		resolveLoopingOverMany("TODO: For expression")
		return nil
	})
}
