package main

import (
	basicunions "github.com/pulumi/pulumi-basic-unions/sdk/v4/go/basic-unions"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// properties field is bound to union case ServerPropertiesForReplica
		_, err := basicunions.NewExampleServer(ctx, "replica", &basicunions.ExampleServerArgs{
			Properties: &basicunions.ServerPropertiesForReplicaArgs{
				CreateMode: pulumi.String("Replica"),
				Version:    pulumi.String("0.1.0-dev"),
			},
		})
		if err != nil {
			return err
		}
		// properties field is bound to union case ServerPropertiesForRestore
		_, err = basicunions.NewExampleServer(ctx, "restore", &basicunions.ExampleServerArgs{
			Properties: &basicunions.ServerPropertiesForRestoreArgs{
				CreateMode:         pulumi.String("PointInTimeRestore"),
				RestorePointInTime: pulumi.String("example"),
			},
		})
		if err != nil {
			return err
		}
		return nil
	})
}
