using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Unknown = Pulumi.Unknown;

return await Deployment.RunAsync(() => 
{
    var provider = new Pulumi.Providers.Unknown("provider");

    var main = new Unknown.Index.Main("main", new()
    {
        First = "hello",
        Second = 
        {
            { "foo", "bar" },
        },
    });

    var fromModule = new List<Unknown.Eks.Example>();
    for (var rangeIndex = 0; rangeIndex < 10; rangeIndex++)
    {
        var range = new { Value = rangeIndex };
        fromModule.Add(new Unknown.Eks.Example($"fromModule-{range.Value}", new()
        {
            AssociatedMain = main.Id,
        }));
    }
    return new Dictionary<string, object?>
    {
        ["mainId"] = main.Id,
        ["values"] = fromModule.Values.First,
    };
});

