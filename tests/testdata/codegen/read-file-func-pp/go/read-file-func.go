package main

import (
	"os"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func readFileOrPanic(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err.Error())
	}
	return string(data)
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		key := readFileOrPanic("key.pub")
		ctx.Export("result", pulumi.String(key))
		return nil
	})
}
