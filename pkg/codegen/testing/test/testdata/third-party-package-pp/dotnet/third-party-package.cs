using System.Collections.Generic;
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

});

