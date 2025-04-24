using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Random = Pulumi.Random;

return await Deployment.RunAsync(() => 
{
    var randomPassword = new Random.RandomPassword("randomPassword", new()
    {
        Length = 16,
        Special = true,
        OverrideSpecial = "_%@",
    });

    return new Dictionary<string, object?>
    {
        ["password"] = randomPassword.Result,
    };
});

