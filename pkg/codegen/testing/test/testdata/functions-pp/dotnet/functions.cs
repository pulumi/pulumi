using System;
using System.Collections.Generic;
using Pulumi;

return await Deployment.RunAsync(() => 
{
    var encoded = Convert.ToBase64String(System.Text.Encoding.UTF8.GetBytes("haha business"));

    var joined = string.Join("-", new[]
    {
        "haha",
        "business",
    });

});

