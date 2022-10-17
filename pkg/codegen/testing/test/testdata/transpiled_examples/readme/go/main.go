package main

import (
	"io/ioutil"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func readFileOrPanic(path string) pulumi.StringPtrInput {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err.Error())
	}
	return pulumi.String(string(data))
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		ctx.Export("strVar", "foo")
		ctx.Export("arrVar", []string{
			"fizz",
			"buzz",
		})
		ctx.Export("readme", readFileOrPanic("./Pulumi.README.md"))
		return nil
	})
}
