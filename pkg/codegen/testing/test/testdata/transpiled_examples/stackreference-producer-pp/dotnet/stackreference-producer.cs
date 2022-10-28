using System.Collections.Generic;
using Pulumi;

return await Deployment.RunAsync(() => 
{
    return new Dictionary<string, object?>
    {
        ["imageName"] = "pulumi/pulumi:latest",
    };
});

