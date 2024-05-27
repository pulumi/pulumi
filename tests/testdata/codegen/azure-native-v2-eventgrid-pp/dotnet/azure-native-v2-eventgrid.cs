using System.Collections.Generic;
using System.Linq;
using Pulumi;
using AzureNative = Pulumi.AzureNative;

return await Deployment.RunAsync(() => 
{
    var example = new AzureNative.EventGrid.EventSubscription("example", new()
    {
        ExpirationTimeUtc = "example",
        Scope = "example",
    });

});

