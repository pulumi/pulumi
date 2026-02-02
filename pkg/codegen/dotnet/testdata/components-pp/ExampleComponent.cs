using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Random = Pulumi.Random;

namespace Components
{
    public class ExampleComponentArgs : global::Pulumi.ResourceArgs
    {
        public class DeploymentZonesArgs : global::Pulumi.ResourceArgs
        {
            [Input("zone")]
            public Input<string>? Zone { get; set; }
        }

        public class GithubAppArgs : global::Pulumi.ResourceArgs
        {
            [Input("id")]
            public Input<string>? Id { get; set; }
            [Input("keyBase64")]
            public Input<string>? KeyBase64 { get; set; }
            [Input("webhookSecret")]
            public Input<string>? WebhookSecret { get; set; }
        }

        public class ServersArgs : global::Pulumi.ResourceArgs
        {
            [Input("name")]
            public Input<string>? Name { get; set; }
        }

        /// <summary>
        /// A simple input
        /// </summary>
        [Input("input")]
        public Input<string> Input { get; set; } = null!;
        /// <summary>
        /// The main CIDR blocks for the VPC
        /// It is a map of strings
        /// </summary>
        [Input("cidrBlocks")]
        public InputMap<string> CidrBlocks { get; set; } = null!;
        /// <summary>
        /// GitHub app parameters, see your github app. Ensure the key is the base64-encoded `.pem` file (the output of `base64 app.private-key.pem`, not the content of `private-key.pem`).
        /// </summary>
        [Input("githubApp")]
        public GithubAppArgs GithubApp { get; set; } = null!;
        /// <summary>
        /// A list of servers
        /// </summary>
        [Input("servers")]
        public ServersArgs[] Servers { get; set; } = null!;
        /// <summary>
        /// A map between for zones
        /// </summary>
        [Input("deploymentZones")]
        public Dictionary<string, DeploymentZonesArgs> DeploymentZones { get; set; } = null!;
        [Input("ipAddress")]
        public InputList<int> IpAddress { get; set; } = null!;
    }

    public class ExampleComponent : global::Pulumi.ComponentResource
    {
        [Output("result")]
        public Output<string> Result { get; private set; }
        public ExampleComponent(string name, ExampleComponentArgs args, ComponentResourceOptions? opts = null)
            : base("components:index:ExampleComponent", name, args, opts)
        {
            var password = new Random.RandomPassword($"{name}-password", new()
            {
                Length = 16,
                Special = true,
                OverrideSpecial = args.Input,
            }, new CustomResourceOptions
            {
                Parent = this,
            });

            var githubPassword = new Random.RandomPassword($"{name}-githubPassword", new()
            {
                Length = 16,
                Special = true,
                OverrideSpecial = args.GithubApp.WebhookSecret,
            }, new CustomResourceOptions
            {
                Parent = this,
            });

            // Example of iterating a list of objects
            var serverPasswords = new List<Random.RandomPassword>();
            for (var rangeIndex = 0; rangeIndex < args.Servers.Length; rangeIndex++)
            {
                var range = new { Value = rangeIndex };
                serverPasswords.Add(new Random.RandomPassword($"{name}-serverPasswords-{range.Value}", new()
                {
                    Length = 16,
                    Special = true,
                    OverrideSpecial = args.Servers[range.Value].Name,
                }, new CustomResourceOptions
                {
                    Parent = this,
                }));
            }
            // Example of iterating a map of objects
            var zonePasswords = new List<Random.RandomPassword>();
            foreach (var range in args.DeploymentZones.Select(pair => new { pair.Key, pair.Value }))
            {
                zonePasswords.Add(new Random.RandomPassword($"{name}-zonePasswords-{range.Key}", new()
                {
                    Length = 16,
                    Special = true,
                    OverrideSpecial = range.Value.Zone,
                }, new CustomResourceOptions
                {
                    Parent = this,
                }));
            }
            var simpleComponent = new Components.SimpleComponent($"{name}-simpleComponent", new ComponentResourceOptions
            {
                Parent = this,
            });

            this.Result = password.Result;

            this.RegisterOutputs(new Dictionary<string, object?>
            {
                ["result"] = password.Result,
            });
        }
    }
}
