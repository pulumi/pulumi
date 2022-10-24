package main

import (
	"fmt"
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
		basicStrVar := "foo"
		ctx.Export("strVar", pulumi.String(basicStrVar))
		ctx.Export("computedStrVar", pulumi.String(fmt.Sprintf("%v/computed", basicStrVar)))
		ctx.Export("strArrVar", pulumi.ToStringArray([]string{
			"fiz",
			"buss",
		}))
		ctx.Export("intVar", pulumi.Float64(42))
		ctx.Export("intArr", pulumi.ToFloat64Array([]float64{
			1,
			2,
			3,
			4,
			5,
		}))
		ctx.Export("readme", readFileOrPanic("./Pulumi.README.md"))
		return nil
	})
}
