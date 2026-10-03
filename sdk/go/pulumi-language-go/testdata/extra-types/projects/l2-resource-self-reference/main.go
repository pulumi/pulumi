package main

import (
	"example.com/pulumi-selfref/sdk/go/v54/selfref"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		root, err := selfref.NewNode(ctx, "root", nil)
		if err != nil {
			return err
		}
		_, err = selfref.NewNode(ctx, "child", &selfref.NodeArgs{
			Parent: root,
			Parents: selfref.NodeArray{
				root,
			},
			NamedParents: selfref.NodeMap{
				"root": root,
			},
			ParentOrName: root,
		})
		if err != nil {
			return err
		}
		return nil
	})
}
