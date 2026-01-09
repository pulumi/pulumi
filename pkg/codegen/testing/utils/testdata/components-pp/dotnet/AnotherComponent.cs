using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Random = Pulumi.Random;

namespace Components
{
    public class AnotherComponent : global::Pulumi.ComponentResource
    {
        public AnotherComponent(string name, ComponentResourceOptions? opts = null)
            : base("components:index:AnotherComponent", name, ResourceArgs.Empty, opts)
        {
            var firstPassword = new Random.RandomPassword($"{name}-firstPassword", new()
            {
                Length = 16,
                Special = true,
            }, new CustomResourceOptions
            {
                Parent = this,
            });

            this.RegisterOutputs();
        }
    }
}
