using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Random = Pulumi.Random;

namespace Components
{
    public class SimpleComponent : global::Pulumi.ComponentResource
    {
        public SimpleComponent(string name, ComponentResourceOptions? opts = null)
            : base("components:index:SimpleComponent", name, ResourceArgs.Empty, opts)
        {
            var firstPassword = new Random.RandomPassword($"{name}-firstPassword", new()
            {
                Length = 16,
                Special = true,
            }, new CustomResourceOptions
            {
                Parent = this,
            });

            var secondPassword = new Random.RandomPassword($"{name}-secondPassword", new()
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
