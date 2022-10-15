using System.Collections.Generic;
using System.IO;
using Pulumi;

return await Deployment.RunAsync(() => 
{
    var basicStrVar = "foo";

    return new Dictionary<string, object?>
    {
        ["strVar"] = basicStrVar,
        ["computedStrVar"] = $"{basicStrVar}/computed",
        ["strArrVar"] = new[]
        {
            "fiz",
            "buss",
        },
        ["intVar"] = 42,
        ["intArr"] = new[]
        {
            1,
            2,
            3,
            4,
            5,
        },
        ["readme"] = File.ReadAllText("./Pulumi.README.md"),
    };
});

