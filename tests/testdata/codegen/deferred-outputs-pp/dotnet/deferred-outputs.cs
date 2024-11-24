using System.Collections.Generic;
using System.Linq;
using Pulumi;

return await Deployment.RunAsync(() => 
{
    var secondPasswordLength, resolveSecondPasswordLength = new Pulumi.DeferredOutput<int>()
    var first = new Components.First("first", new()
    {
        PasswordLength = secondPasswordLength,
    });

    var second = new Components.Second("second", new()
    {
        PetName = first.PetName,
    });

    resolveSecondPasswordLength(second.PasswordLength);
    var loopingOverMany, resolveLoopingOverMany = new Pulumi.DeferredOutput<List<int>>()
    var another = new Components.First("another", new()
    {
        PasswordLength = loopingOverMany.Length,
    });

    var many = new List<Components.Second>();
    for (var rangeIndex = 0; rangeIndex < 10; rangeIndex++)
    {
        var range = new { Value = rangeIndex };
        many.Add(new Components.Second($"many-{range.Value}", new()
        {
            PetName = another.PetName,
        }));
    }
    resolveLoopingOverMany(Output.Create(many.Select((value, i) => new { Key = i.ToString(), Value = pair.Value }).Select(v => 
    {
        return v.PasswordLength;
    }).ToList()));
});

