using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Other = ThirdParty.Other;

return await Deployment.RunAsync(() => 
{
    var Other = new Other.Thing("Other", new()
    {
        Idea = "Support Third Party",
    });

    var Question = new Other.Module.Object("Question", new()
    {
        Answer = 42,
    });

    var Question2 = new Other.Module.Sub.Object("Question2", new()
    {
        Answer = 24,
    });

    var Provider = new Other.Provider("Provider", new()
    {
        ObjectProp = 
        {
            { "prop1", "foo" },
            { "prop2", "bar" },
            { "prop3", "fizz" },
        },
    });

});

