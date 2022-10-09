using System.Collections.Generic;
using System.IO;
using Pulumi;

return await Deployment.RunAsync(() => 
{
    var key = File.ReadAllText("key.pub");

    return new Dictionary<string, object?>
    {
        ["result"] = key,
    };
});

