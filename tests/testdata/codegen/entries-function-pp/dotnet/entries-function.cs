using System.Collections.Generic;
using System.Linq;
using Pulumi;

return await Deployment.RunAsync(() => 
{
    var data = new[]
    {
        1,
        2,
        3,
    }.Select((v, k) => new { Key = k, Value = v }).Select(entry => 
    {
        return 
        {
            { "usingKey", entry.Key },
            { "usingValue", entry.Value },
        };
    }).ToList();

});

