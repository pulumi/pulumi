using System.Collections.Generic;
using System.Linq;
using Pulumi;
using AwsStaticWebsite = Pulumi.AwsStaticWebsite;

return await Deployment.RunAsync(() => 
{
    var websiteResource = new AwsStaticWebsite.Website("websiteResource", new()
    {
        SitePath = "string",
        IndexHTML = "string",
        CacheTTL = 0,
        CdnArgs = new AwsStaticWebsite.Inputs.CDNArgsArgs
        {
            CloudfrontFunctionAssociations = new[]
            {
                new Aws.CloudFront.Inputs.DistributionOrderedCacheBehaviorFunctionAssociationArgs
                {
                    EventType = "string",
                    FunctionArn = "string",
                },
            },
            ForwardedValues = new Aws.CloudFront.Inputs.DistributionDefaultCacheBehaviorForwardedValuesArgs
            {
                Cookies = new Aws.CloudFront.Inputs.DistributionDefaultCacheBehaviorForwardedValuesCookiesArgs
                {
                    Forward = "string",
                    WhitelistedNames = new[]
                    {
                        "string",
                    },
                },
                QueryString = false,
                Headers = new[]
                {
                    "string",
                },
                QueryStringCacheKeys = new[]
                {
                    "string",
                },
            },
            LambdaFunctionAssociations = new[]
            {
                new Aws.CloudFront.Inputs.DistributionOrderedCacheBehaviorLambdaFunctionAssociationArgs
                {
                    EventType = "string",
                    LambdaArn = "string",
                    IncludeBody = false,
                },
            },
        },
        CertificateARN = "string",
        Error404 = "string",
        AddWebsiteVersionHeader = false,
        PriceClass = "string",
        AtomicDeployments = false,
        Subdomain = "string",
        TargetDomain = "string",
        WithCDN = false,
        WithLogs = false,
    });

});

