using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Unknown = Pulumi.Unknown;

return await Deployment.RunAsync(() => 
{
    var data = Unknown.Index.GetData.Invoke(new()
    {
        Input = "hello",
    });

    var values = Unknown.Eks.ModuleValues.Invoke();

    return new Dictionary<string, object?>
    {
        ["content"] = data.Content,
    };
});

