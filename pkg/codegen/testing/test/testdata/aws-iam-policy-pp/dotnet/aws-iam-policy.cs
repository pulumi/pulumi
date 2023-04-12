using System.Collections.Generic;
using System.Linq;
using System.Text.Json;
using Pulumi;
using Aws = Pulumi.Aws;

return await Deployment.RunAsync(() => 
{
    // Create a policy with multiple Condition keys
    var policy = new Aws.Iam.Policy("policy", new()
    {
        Path = "/",
        Description = "My test policy",
        PolicyDocument = JsonSerializer.Serialize(new Dictionary<string, object?>
        {
            ["Version"] = "2012-10-17",
            ["Statement"] = new[]
            {
                new Dictionary<string, object?>
                {
                    ["Effect"] = "Allow",
                    ["Action"] = "lambda:*",
                    ["Resource"] = "arn:aws:lambda:*:*:function:*",
                    ["Condition"] = new Dictionary<string, object?>
                    {
                        ["StringEquals"] = new Dictionary<string, object?>
                        {
                            ["aws:RequestTag/Team"] = new[]
                            {
                                "iamuser-admin",
                                "iamuser2-admin",
                            },
                        },
                        ["ForAllValues:StringEquals"] = new Dictionary<string, object?>
                        {
                            ["aws:TagKeys"] = new[]
                            {
                                "Team",
                            },
                        },
                    },
                },
            },
        }),
    });

    return new Dictionary<string, object?>
    {
        ["policyName"] = policy.Name,
    };
});

