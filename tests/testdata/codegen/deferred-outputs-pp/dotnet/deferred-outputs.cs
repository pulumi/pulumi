using System.Collections.Generic;
using System.Linq;
using Pulumi;

return await Deployment.RunAsync(() => 
{
    var secondPasswordLength = new Pulumi.DeferredOutput<int>();
    var first = new Components.First("first", new()
    {
        PasswordLength = secondPasswordLength.Output,
    });

    var second = new Components.Second("second", new()
    {
        PetName = first.PetName,
    });

    secondPasswordLength.Resolve(second.PasswordLength);
    var loopingOverMany = new Pulumi.DeferredOutput<List<int>>();
    var another = new Components.First("another", new()
    {
        PasswordLength = loopingOverMany.Output.Apply(loopingOverMany => loopingOverMany.Length),
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
    loopingOverMany.Resolve(Output.Create(many.Select((value, i) => new { Key = i.ToString(), Value = pair.Value }).Select(v => 
    {
        return v.PasswordLength;
    }).ToList()));
});

