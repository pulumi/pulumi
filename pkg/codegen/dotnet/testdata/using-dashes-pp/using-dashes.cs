using System.Collections.Generic;
using System.Linq;
using Pulumi;
using UsingDashes = Pulumi.UsingDashes;

return await Deployment.RunAsync(() => 
{
    var main = new UsingDashes.Dash("main", new()
    {
        Stack = "dev",
    });

});

