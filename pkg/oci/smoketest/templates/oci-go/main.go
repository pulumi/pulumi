// A minimal OCI-mode Pulumi program. It runs as a container (runtime: oci), built from
// the Dockerfile in this directory. Add resources as usual; run `pulumi package add
// <provider>` to generate and wire a provider SDK.
package main

import "github.com/pulumi/pulumi/sdk/v3/go/pulumi"

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		ctx.Export("greeting", pulumi.String("hello from ${PROJECT}, an OCI go program"))
		return nil
	})
}
