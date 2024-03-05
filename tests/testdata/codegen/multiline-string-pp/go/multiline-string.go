package main

import (
	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := random.NewRandomShuffle(ctx, "foo", &random.RandomShuffleArgs{
			Inputs: pulumi.StringArray{
				pulumi.String("just one\nnewline"),
				pulumi.String("foo\nbar\nbaz\nqux\nquux\nqux"),
				pulumi.String(`{
    "a": 1,
    "b": 2,
    "c": [
      "foo",
      "bar",
      "baz",
      "qux",
      "quux"
    ]
}
`),
			},
		})
		if err != nil {
			return err
		}
		return nil
	})
}
