using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Random = Pulumi.Random;

return await Deployment.RunAsync(() => 
{
    var config = new Config();
    var configLexicalName = config.Require("cC-Charlie_charlie.😃⁉️");
    var resourceLexicalName = new Random.RandomPet("aA-Alpha_alpha.🤯⁉️", new()
    {
        Prefix = configLexicalName,
    });

    return new Dictionary<string, object?>
    {
        ["bB-Beta_beta.💜⁉"] = resourceLexicalName.Id,
        ["dD-Delta_delta.🔥⁉"] = resourceLexicalName.Id,
    };
});

