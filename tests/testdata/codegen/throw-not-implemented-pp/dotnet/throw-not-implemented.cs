using System.Collections.Generic;
using System.Linq;
using Pulumi;

	
object NotImplemented(string errorMessage) 
{
    throw new System.NotImplementedException(errorMessage);
}

return await Deployment.RunAsync(() => 
{
    return new Dictionary<string, object?>
    {
        ["result"] = NotImplemented("expression here is not implemented yet"),
    };
});

