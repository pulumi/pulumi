using System.Collections.Generic;
using Pulumi;
using Random = Pulumi.Random;

await Deployment.RunAsync(() => 
{
    var resourceLexicalName = new Random.RandomPet("aA-Alpha_alpha.🤯⁉️");

    return new Dictionary<string, object?>
    {
        ["bB-Beta_beta.💜⁉"] = resourceLexicalName.Id,
    };
});

