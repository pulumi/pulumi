using System.Collections.Generic;
using System.Linq;
using Pulumi;
using AwsNative = Pulumi.AwsNative;

return await Deployment.RunAsync(() => 
{
    var role = new AwsNative.Iam.Role("role", new()
    {
        RoleName = "ScriptIAMRole",
        AssumeRolePolicyDocument = new Dictionary<string, object?>
        {
            ["Version"] = "2012-10-17",
            ["Statement"] = new[]
            {
                new Dictionary<string, object?>
                {
                    ["Effect"] = "Allow",
                    ["Action"] = "sts:AssumeRole",
                    ["Principal"] = new Dictionary<string, object?>
                    {
                        ["Service"] = new[]
                        {
                            "cloudformation.amazonaws.com",
                            "gamelift.amazonaws.com",
                        },
                    },
                },
            },
        },
    });

});

