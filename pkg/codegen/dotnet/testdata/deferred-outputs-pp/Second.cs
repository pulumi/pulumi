using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Random = Pulumi.Random;

namespace Components
{
    public class SecondArgs : global::Pulumi.ResourceArgs
    {
        [Input("petName")]
        public Input<string> PetName { get; set; } = null!;
    }

    public class Second : global::Pulumi.ComponentResource
    {
        [Output("passwordLength")]
        public Output<int> PasswordLength { get; private set; }
        public Second(string name, SecondArgs args, ComponentResourceOptions? opts = null)
            : base("components:index:Second", name, args, opts)
        {
            var randomPet = new Random.RandomPet($"{name}-randomPet", new()
            {
                Length = args.PetName.Length,
            }, new CustomResourceOptions
            {
                Parent = this,
            });

            var password = new Random.RandomPassword($"{name}-password", new()
            {
                Length = 16,
                Special = true,
                Numeric = false,
            }, new CustomResourceOptions
            {
                Parent = this,
            });

            this.PasswordLength = password.Length;

            this.RegisterOutputs(new Dictionary<string, object?>
            {
                ["passwordLength"] = password.Length,
            });
        }
    }
}
