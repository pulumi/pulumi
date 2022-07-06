using System.Collections.Generic;
using System.Text.Json;
using Pulumi;
using Aws = Pulumi.Aws;

class MyStack : Stack
{
    public MyStack()
    {
        // Create a policy with multiple Condition keys
        var policy = new Aws.Iam.Policy("policy", new Aws.Iam.PolicyArgs
        {
            Path = "/",
            Description = "My test policy",
            PolicyDocument = JsonSerializer.Serialize(new Dictionary<string, object?>
            {
                { "Version", "2012-10-17" },
                { "Statement", new[]
                    {
                        new Dictionary<string, object?>
                        {
                            { "Effect", "Allow" },
                            { "Principal", "*" },
                            { "Action", new[]
                                {
                                    "s3:GetObject",
                                }
                             },
                            { "Resource", new[]
                                {
                                    "arn:aws:s3:::some-aws-bucket/*",
                                }
                             },
                            { "Condition", new Dictionary<string, object?>
                            {
                                { "Foo", new Dictionary<string, object?>
                                {
                                    { "Bar", new[]
                                        {
                                            "iamuser-admin",
                                            "iamuser2-admin",
                                        }
                                     },
                                } },
                                { "Baz", new Dictionary<string, object?>
                                {
                                    { "Qux", new[]
                                        {
                                            "iamuser3-admin",
                                        }
                                     },
                                } },
                            } },
                        },
                    }
                 },
            }),
        });
    }

}
