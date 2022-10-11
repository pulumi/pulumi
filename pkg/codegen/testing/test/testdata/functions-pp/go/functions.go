package main

import (
	"encoding/base64"
	"strings"

	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/s3"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		encoded := base64.StdEncoding.EncodeToString([]byte("haha business"))
		tmpVar0, _ := base64.StdEncoding.DecodeString(encoded)
		decoded := string(tmpVar0)
		_ = strings.Join([]string{
			encoded,
			decoded,
			"2",
		}, "-")
		bucket, err := s3.NewBucket(ctx, "bucket", nil)
		if err != nil {
			return err
		}
		_ = bucket.ID().ApplyT(func(id string) (pulumi.String, error) {
			return pulumi.String(base64.StdEncoding.EncodeToString([]byte(id))), nil
		}).(pulumi.StringOutput)
		_ = bucket.ID().ApplyT(func(id string) (pulumi.String, error) {
			value, _ := base64.StdEncoding.DecodeString(id)
			return pulumi.String(value), nil
		}).(pulumi.StringOutput)
		return nil
	})
}
