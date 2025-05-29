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
			IndexHTML: "string",
			CacheTTL:  0,
			CdnArgs: &*awsstaticwebsite.CDNArgsArgs{
				CloudfrontFunctionAssociations: []cloudfront.DistributionOrderedCacheBehaviorFunctionAssociationArgs{
					{
						EventType:   pulumi.String("string"),
						FunctionArn: pulumi.String("string"),
					},
				},
				ForwardedValues: &*cloudfront.DistributionDefaultCacheBehaviorForwardedValuesArgs{
					Cookies: &cloudfront.DistributionDefaultCacheBehaviorForwardedValuesCookiesArgs{
						Forward: pulumi.String("string"),
						WhitelistedNames: []pulumi.String{
							pulumi.String("string"),
						},
					},
					QueryString: pulumi.Bool(false),
					Headers: []pulumi.String{
						pulumi.String("string"),
					},
					QueryStringCacheKeys: []pulumi.String{
						pulumi.String("string"),
					},
				},
				LambdaFunctionAssociations: []cloudfront.DistributionOrderedCacheBehaviorLambdaFunctionAssociationArgs{
					{
						EventType:   pulumi.String("string"),
						LambdaArn:   pulumi.String("string"),
						IncludeBody: false,
					},
				},
			},
			CertificateARN:          "string",
			Error404:                "string",
			AddWebsiteVersionHeader: false,
			PriceClass:              "string",
			AtomicDeployments:       false,
			Subdomain:               "string",
			TargetDomain:            "string",
			WithCDN:                 false,
			WithLogs:                false,
		})
		if err != nil {
			return err
		}
		return nil
	})
}
