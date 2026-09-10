package main

import (
	"example.com/pulumi-bytesink/sdk/go/v47/bytesink"
	"example.com/pulumi-bytesource/sdk/go/v48/bytesource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		source, err := bytesource.NewResource(ctx, "source", &bytesource.ResourceArgs{
			Base64: pulumi.String("AGhlbGxvIID+/yB3b3JsZPAo"),
		})
		if err != nil {
			return err
		}
		_, err = bytesink.NewResource(ctx, "sink", &bytesink.ResourceArgs{
			Bytes:        source.Bytes,
			ExpectBase64: source.Base64,
		})
		if err != nil {
			return err
		}
		return nil
	})
}
