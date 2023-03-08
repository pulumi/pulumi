using System.Collections.Generic;
using Pulumi;

return await Deployment.RunAsync(() => 
{
    var simpleComponent = new Components.SimpleComponent("simpleComponent");

    var exampleComponent = new Components.ExampleComponent("exampleComponent", new()
    {
        Input = "doggo",
    });

    return new Dictionary<string, object?>
    {
        ["result"] = exampleComponent.Result,
    };
});

