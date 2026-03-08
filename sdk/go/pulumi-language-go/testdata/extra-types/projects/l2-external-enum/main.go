package main

import (
	"example.com/pulumi-enum/sdk/go/v30/enum"
	"example.com/pulumi-extenumref/sdk/go/v32/extenumref"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := enum.NewRes(ctx, "myRes", &enum.ResArgs{
			IntEnum:    enum.IntEnumIntOne,
			StringEnum: enum.StringEnumStringOne,
		})
		if err != nil {
			return err
		}
		_, err = extenumref.NewSink(ctx, "mySink", &extenumref.SinkArgs{
			StringEnum: enum.StringEnumStringTwo,
		})
		if err != nil {
			return err
		}
		return nil
	})
}
