using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Std = Pulumi.Std;

return await Deployment.RunAsync(() => 
{
    var example = Std.Replace.Invoke(new()
    {
        Text = Std.Upper.Invoke(new()
        {
            Input = "hello_world",
        }).Result,
        Search = "_",
        Replace = "-",
    });

});

