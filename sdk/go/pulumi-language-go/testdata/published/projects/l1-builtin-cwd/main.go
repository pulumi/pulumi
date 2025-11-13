package main

import (
	"os"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		ctx.Export("cwdOutput", pulumi.String(func(cwd string, err error) string {
			if err != nil {
				panic(err)
			}
			return cwd
		}(os.Getwd())))
		return nil
	})
}
