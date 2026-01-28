using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Random = Pulumi.Random;

namespace Components
{
    public class FirstArgs : global::Pulumi.ResourceArgs
    {
        [Input("passwordLength")]
        public Input<double> PasswordLength { get; set; } = null!;
    }

    public class First : global::Pulumi.ComponentResource
    {
        [Output("petName")]
        public Output<string> PetName { get; private set; }
        [Output("password")]
        public Output<string> Password { get; private set; }
        public First(string name, FirstArgs args, ComponentResourceOptions? opts = null)
            : base("components:index:First", name, args, opts)
        {
            var randomPet = new Random.RandomPet($"{name}-randomPet", new()
            {
            }, new CustomResourceOptions
            {
                Parent = this,
            });

            var randomPassword = new Random.RandomPassword($"{name}-randomPassword", new()
            {
                Length = args.PasswordLength,
            }, new CustomResourceOptions
            {
                Parent = this,
            });

            this.PetName = randomPet.Id;
            this.Password = randomPassword.Result;

            this.RegisterOutputs(new Dictionary<string, object?>
            {
                ["petName"] = randomPet.Id,
                ["password"] = randomPassword.Result,
            });
        }
    }
}
