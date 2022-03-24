// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

using System.Collections.Generic;
using System.Collections.Immutable;
using System.Threading.Tasks;
using Pulumi;

public sealed class RandomResourceProvider : DynamicResourceProvider
{
    public override async Task<(string, IDictionary<string, object>)> Create(ImmutableDictionary<string, object> properties)
    {
        var random = new System.Random();
        var buffer = new byte[15];
        random.NextBytes(buffer);
        var val = System.Convert.ToBase64String(buffer);

        return (val, new Dictionary<string, object>{ { "val", val } });
    }
}

public sealed class RandomArgs : DynamicResourceArgs
{
    [Input("val")]
    public Input<string> Val { get; set; }
}

class Random : DynamicResource
{
    [Output("val")]
    public Output<string> Val { get; private set; }

    public Random(string name, CustomResourceOptions options = null)
        : base(new RandomResourceProvider(), name, new RandomArgs() { Val = "" }, options)
    {
    }
}

class Program
{
    static Task<int> Main(string[] args)
    {
        return Deployment.RunAsync(() => {
            var r = new Random("foo");

            return new Dictionary<string, object> {
                { "random_id", r.Id },
                { "random_val", r.Val }
            };
        });
    }
}