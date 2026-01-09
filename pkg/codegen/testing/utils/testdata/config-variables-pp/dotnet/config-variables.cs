using System.Collections.Generic;
using System.Linq;
using Pulumi;

return await Deployment.RunAsync(() => 
{
    var config = new Config();
    var requiredString = config.Require("requiredString");
    var requiredInt = config.RequireInt32("requiredInt");
    var requiredFloat = config.RequireDouble("requiredFloat");
    var requiredBool = config.RequireBoolean("requiredBool");
    var requiredAny = config.RequireObject<dynamic>("requiredAny");
    var optionalString = config.Get("optionalString") ?? "defaultStringValue";
    var optionalInt = config.GetInt32("optionalInt") ?? 42;
    var optionalFloat = config.GetDouble("optionalFloat") ?? 3.14;
    var optionalBool = config.GetBoolean("optionalBool") ?? true;
    var optionalAny = config.GetObject<dynamic>("optionalAny") ?? 
    {
        { "key", "value" },
    };
});

