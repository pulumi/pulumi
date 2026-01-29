using System.Collections.Generic;
using System.Linq;
using Pulumi;

return await Deployment.RunAsync(() => 
{
    var config = new Config();
    var cidrBlock = config.Get("cidrBlock") ?? "Test config variable";
    return new Dictionary<string, object?>
    {
        ["cidrBlock"] = cidrBlock,
    };
});

