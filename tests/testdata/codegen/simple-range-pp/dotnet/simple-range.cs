using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Random = Pulumi.Random;

return await Deployment.RunAsync(() => 
{
    var numbers = new List<Random.RandomInteger>();
    for (var rangeIndex = 0; rangeIndex < 2; rangeIndex++)
    {
        var range = new { Value = rangeIndex };
        numbers.Add(new Random.RandomInteger($"numbers-{range.Value}", new()
        {
            Min = 1,
            Max = range.Value,
            Seed = $"seed{range.Value}",
        }));
    }
    return new Dictionary<string, object?>
    {
        ["first"] = numbers[0].Id,
        ["second"] = numbers[1].Id,
    };
});

