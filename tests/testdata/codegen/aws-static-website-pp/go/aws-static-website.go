package main

import (
	awsstaticwebsite "github.com/pulumi/pulumi-aws-static-website/sdk/go/aws-static-website"
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/cloudfront"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := awsstaticwebsite.NewWebsite(ctx, "websiteResource", &awsstaticwebsite.WebsiteArgs{
			SitePath:  pulumi.String("string"),
			IndexHTML: pulumi.String("string"),
			CacheTTL:  pulumi.Float64(0),
			CdnArgs: &awsstaticwebsite.CDNArgsArgs{
				CloudfrontFunctionAssociations: cloudfront.DistributionOrderedCacheBehaviorFunctionAssociationArray{
					&cloudfront.DistributionOrderedCacheBehaviorFunctionAssociationArgs{
						EventType:   pulumi.String("string"),
						FunctionArn: pulumi.String("string"),
					},
				},
				ForwardedValues: &cloudfront.DistributionDefaultCacheBehaviorForwardedValuesArgs{
					Cookies: &cloudfront.DistributionDefaultCacheBehaviorForwardedValuesCookiesArgs{
						Forward: pulumi.String("string"),
						WhitelistedNames: pulumi.StringArray{
							pulumi.String("string"),
						},
					},
					QueryString: pulumi.Bool(false),
					Headers: pulumi.StringArray{
						pulumi.String("string"),
					},
					QueryStringCacheKeys: pulumi.StringArray{
						pulumi.String("string"),
					},
				},
				LambdaFunctionAssociations: cloudfront.DistributionOrderedCacheBehaviorLambdaFunctionAssociationArray{
					&cloudfront.DistributionOrderedCacheBehaviorLambdaFunctionAssociationArgs{
						EventType:   pulumi.String("string"),
						LambdaArn:   pulumi.String("string"),
						IncludeBody: pulumi.Bool(false),
					},
				},
			},
			CertificateARN:          pulumi.String("string"),
			Error404:                pulumi.String("string"),
			AddWebsiteVersionHeader: pulumi.Bool(false),
			PriceClass:              pulumi.String("string"),
			AtomicDeployments:       pulumi.Bool(false),
			Subdomain:               pulumi.String("string"),
			TargetDomain:            pulumi.String("string"),
			WithCDN:                 pulumi.Bool(false),
			WithLogs:                pulumi.Bool(false),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
