package main

import (
	"example.com/pulumi-pkg/sdk/go/pkg"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := pkg.NewRandom(ctx, "random", &pkg.RandomArgs{
			Length: pulumi.Int(8),
		})
		if err != nil {
			return err
		}

		hello := "hello"
		_, err = pkg.DoEcho(ctx, &pkg.DoEchoArgs{
			Echo: &hello,
		})
		if err != nil {
			return err
		}

		_ = pkg.DoEchoOutput(ctx, pkg.DoEchoOutputArgs{
			Echo: pulumi.String("hello"),
		})

		p, err := pkg.NewEcho(ctx, "echo", &pkg.EchoArgs{})
		if err != nil {
			return err
		}

		_, err = p.DoEchoMethod(ctx, &pkg.EchoDoEchoMethodArgs{
			Echo: pulumi.String("hello"),
		})
		return err
	})
}
