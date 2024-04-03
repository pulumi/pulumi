using System.Collections.Generic;
using System.Linq;
using Pulumi;
using AzureNative = Pulumi.AzureNative;

return await Deployment.RunAsync(() => 
{
    var namespaceAuthorizationRule = new AzureNative.ServiceBus.NamespaceAuthorizationRule("namespaceAuthorizationRule", new()
    {
        AuthorizationRuleName = "sdk-AuthRules-1788",
        NamespaceName = "sdk-Namespace-6914",
        ResourceGroupName = "ArunMonocle",
        Rights = new[]
        {
            AzureNative.ServiceBus.AccessRights.Listen,
            AzureNative.ServiceBus.AccessRights.Send,
        },
    });

});

