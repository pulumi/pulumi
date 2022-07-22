using System.Collections.Generic;
using Pulumi;
using Random = Pulumi.Random;

return await Deployment.RunAsync(() => 
{
    var random_pet = new Random.RandomPet("random-pet", new()
    {
        Prefix = "doggo",
    });

});

