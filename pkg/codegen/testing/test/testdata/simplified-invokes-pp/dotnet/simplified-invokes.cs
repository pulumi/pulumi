using System.Collections.Generic;
using Pulumi;
using Std = Pulumi.Std;

return await Deployment.RunAsync(() => 
{
    var everyArg = Std.AbsMultiArgs.Invoke(10, 20, 30);

    var onlyRequiredArgs = Std.AbsMultiArgs.Invoke(10);

    var optionalArgs = Std.AbsMultiArgs.Invoke(10, null, 30);

});

