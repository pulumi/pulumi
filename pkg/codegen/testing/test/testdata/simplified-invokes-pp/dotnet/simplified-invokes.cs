using System.Collections.Generic;
using Pulumi;
using Std = Pulumi.Std;

return await Deployment.RunAsync(() => 
{
    var everyArg = Std.AbsMultiArgs.Invoke(10, 20, 30);

    var onlyRequiredArgs = Std.AbsMultiArgs.Invoke(10);

    var optionalArgs = Std.AbsMultiArgs.Invoke(10, null, 30);

    var nestedUse = Output.Tuple(everyArg, Std.AbsMultiArgs.Invoke(42)).Apply(values =>
    {
        var everyArg = values.Item1;
        var invoke = values.Item2;
        return Std.AbsMultiArgs.Invoke(everyArg, invoke);
    });

    return new Dictionary<string, object?>
    {
        ["result"] = nestedUse,
    };
});

