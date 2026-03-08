package main

import (
	"example.com/pulumi-enum/sdk/go/v30/enum"
	"example.com/pulumi-enum/sdk/go/v30/enum/mod"
	"example.com/pulumi-enum/sdk/go/v30/enum/mod/nested"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := enum.NewRes(ctx, "sink1", &enum.ResArgs{
			IntEnum:    enum.IntEnumIntOne,
			StringEnum: enum.StringEnumStringTwo,
		})
		if err != nil {
			return err
		}
		_, err = mod.NewRes(ctx, "sink2", &mod.ResArgs{
			IntEnum:    mod.IntEnumIntOne,
			StringEnum: mod.StringEnumStringTwo,
		})
		if err != nil {
			return err
		}
		_, err = nested.NewRes(ctx, "sink3", &nested.ResArgs{
			IntEnum:    nested.IntEnumIntOne,
			StringEnum: nested.StringEnumStringTwo,
		})
		if err != nil {
			return err
		}
		return nil
	})
}
