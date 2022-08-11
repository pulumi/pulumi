// Copyright 2016-2020, Pulumi Corporation.  All rights reserved.

using System.Collections.Generic;
using System.Threading.Tasks;
using Pulumi;
using Pulumi.Random;

class GetResource : CustomResource
{
    [Output("secret")]
    public Output<int> Secret { get; private set; } = null!;

    [Output("length")]
    public Output<int> Length { get; private set; } = null!;

    public GetResource(string urn)
        : base("unused:unused:unused", "unused", ResourceArgs.Empty, new CustomResourceOptions {Urn = urn})
    {
    }
}

class Program
{
    static Task<int> Main(string[] args)
    {
        return Deployment.RunAsync(() =>
        {
            var pet = new RandomPet("cat", new RandomPetArgs {
                Length = 2,
            });

            var getPetLength = pet.Urn.Apply(urn => new GetResource(urn).Length);
            var secretPet = new RandomPet("secretPet", new RandomPetArgs {
                Length = Output.CreateSecret(1),
            });

            var getPetSecretLength = secretPet.Urn.Apply(urn => new GetResource(urn).Length);
            
            return new Dictionary<string, object>
            {
                {"getPetLength", getPetLength},
                {"secret", getPetSecretLength},
            };
        });
    }
}
