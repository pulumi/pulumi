package main

import (
	"fmt"
	"strconv"

	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/s3"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func parseInt(input string) int {
	value, err := strconv.Atoi(input)
	if err != nil {
		panic(err)
	}
	return value
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")
		// Number of AZs to cover in a given region
		azCount := "10"
		if param := cfg.Get("azCount"); param != "" {
			azCount = param
		}
		var bucketsPerAvailabilityZone []*s3.Bucket
		for index := 0; index < parseInt(azCount); index++ {
			key0 := index
			__res, err := s3.NewBucket(ctx, fmt.Sprintf("bucketsPerAvailabilityZone-%v", key0), &s3.BucketArgs{
				Website: &s3.BucketWebsiteArgs{
					IndexDocument: pulumi.String("index.html"),
				},
			})
			if err != nil {
				return err
			}
			bucketsPerAvailabilityZone = append(bucketsPerAvailabilityZone, __res)
		}
		return nil
	})
}
