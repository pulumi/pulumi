using System.Collections.Generic;
using System.Linq;
using Pulumi;

return await Deployment.RunAsync(() => 
{
    return new Dictionary<string, object?>
    {
        ["result"] = Enumerable.Single(new[]
        {
            1,
        }),
    };
});

