using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Std = Pulumi.Std;

return await Deployment.RunAsync(() => 
{
    var everyArg = Std.AbsMultiArgs.Invoke(10, 20, 30);

    var onlyRequiredArgs = Std.AbsMultiArgs.Invoke(10);

    var optionalArgs = Std.AbsMultiArgs.Invoke(10, null, 30);

    var nestedUse = Std.AbsMultiArgs.Invoke(everyArg, Std.AbsMultiArgs.Invoke(42));

    return new Dictionary<string, object?>
    {
        ["result"] = nestedUse,
    };
});

